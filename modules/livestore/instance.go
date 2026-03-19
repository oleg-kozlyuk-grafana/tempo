package livestore

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/grafana/tempo/modules/ingester"
	"github.com/grafana/tempo/modules/overrides"
	"github.com/grafana/tempo/pkg/livetraces"
	"github.com/grafana/tempo/pkg/tempopb"
	v1 "github.com/grafana/tempo/pkg/tempopb/trace/v1"
	"github.com/grafana/tempo/pkg/tracesizes"
	util_log "github.com/grafana/tempo/pkg/util/log"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/backend/local"
	"github.com/grafana/tempo/tempodb/encoding"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	traceDataType              = "trace"
	reasonWaitingForLiveTraces = "waiting_for_live_traces"
	maxTraceLogLinesPerSecond  = 10
)

var (
	// Instance-level metrics (similar to ingester instance.go)
	metricTracesCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "tempo_live_store",
		Name:      "traces_created_total",
		Help:      "The total number of traces created per tenant.",
	}, []string{"tenant"})
	metricLiveTraces = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "tempo_live_store",
		Name:      "live_traces",
		Help:      "The current number of live traces per tenant.",
	}, []string{"tenant"})
	metricLiveTraceBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "tempo_live_store",
		Name:      "live_trace_bytes",
		Help:      "The current number of bytes consumed by live traces per tenant.",
	}, []string{"tenant"})
	metricBytesReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "tempo_live_store",
		Name:      "bytes_received_total",
		Help:      "The total bytes received per tenant.",
	}, []string{"tenant", "data_type"})
	metricBlocksClearedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "tempo_live_store",
		Name:      "blocks_cleared_total",
		Help:      "The total number of blocks cleared.",
	}, []string{"block_type"})
	metricCompletionSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "tempo_live_store",
		Name:      "completion_size_bytes",
		Help:      "Size in bytes of blocks completed.",
		Buckets:   prometheus.ExponentialBuckets(1024*1024, 2, 10), // from 1MB up to 1GB
	})
	metricBackPressure = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "tempo",
		Subsystem: "live_store",
		Name:      "back_pressure_seconds_total",
		Help:      "The total amount of time spent waiting to process data from queue",
	}, []string{"reason"})
)

// bufferedTrace holds a cut trace waiting to be written to a complete block.
type bufferedTrace struct {
	ID        []byte
	Trace     *tempopb.Trace
	StartSecs uint32
	EndSecs   uint32
}

type instance struct {
	tenantID string
	logger   log.Logger

	// Configuration
	Cfg Config

	// Local backend for complete block storage
	localBackend          *local.Backend
	completeBlockEncoding encoding.VersionedEncoding

	// Block management — blocksMtx also protects traceBuffer
	blocksMtx        sync.RWMutex
	traceBuffer      []*bufferedTrace
	traceBufferBytes uint64
	traceBufferStart time.Time
	completeBlocks   map[uuid.UUID]*ingester.LocalBlock

	// Live traces
	liveTracesMtx  sync.Mutex
	liveTraces     *livetraces.LiveTraces[*v1.ResourceSpans]
	traceSizes     *tracesizes.Tracker
	maxTraceLogger *util_log.RateLimitedLogger

	// Metrics
	tracesCreatedTotal prometheus.Counter
	bytesReceivedTotal *prometheus.CounterVec

	overrides overrides.Interface
}

func newInstance(instanceID string, cfg Config, localBackend *local.Backend, completeBlockEncoding encoding.VersionedEncoding, overrides overrides.Interface, logger log.Logger) (*instance, error) {
	logger = log.With(logger, "tenant", instanceID)

	i := &instance{
		tenantID:              instanceID,
		logger:                logger,
		Cfg:                   cfg,
		localBackend:          localBackend,
		completeBlockEncoding: completeBlockEncoding,
		completeBlocks:        map[uuid.UUID]*ingester.LocalBlock{},
		liveTraces:            livetraces.New[*v1.ResourceSpans](func(rs *v1.ResourceSpans) uint64 { return uint64(rs.Size()) }, cfg.MaxTraceIdle, cfg.MaxTraceLive, instanceID),
		traceSizes:            tracesizes.New(),
		maxTraceLogger:        util_log.NewRateLimitedLogger(maxTraceLogLinesPerSecond, level.Warn(logger)),
		overrides:             overrides,
		tracesCreatedTotal:    metricTracesCreatedTotal.WithLabelValues(instanceID),
		bytesReceivedTotal:    metricBytesReceivedTotal,
	}

	return i, nil
}

func (i *instance) backpressure(ctx context.Context) bool {
	if i.Cfg.MaxLiveTracesBytes > 0 {
		// Check live traces
		i.liveTracesMtx.Lock()
		sz := i.liveTraces.Size()
		i.liveTracesMtx.Unlock()

		if sz >= i.Cfg.MaxLiveTracesBytes {
			// Live traces exceeds the expected amount of data per flush, so wait a bit.
			select {
			case <-ctx.Done():
				return false
			case <-time.After(1 * time.Second):
			}

			metricBackPressure.WithLabelValues(reasonWaitingForLiveTraces).Inc()
			return true
		}
	}

	return false
}

func (i *instance) pushBytes(ctx context.Context, ts time.Time, req *tempopb.PushBytesRequest) {
	if len(req.Traces) != len(req.Ids) {
		level.Error(i.logger).Log("msg", "mismatched traces and ids length", "IDs", len(req.Ids), "traces", len(req.Traces))
		return
	}

	// Wait for room in pipeline if needed
	for i.backpressure(ctx) {
	}

	if err := ctx.Err(); err != nil {
		level.Error(i.logger).Log("msg", "failed to push bytes to instance", "err", err)
		return
	}

	// Check tenant limits
	maxBytes := i.overrides.MaxBytesPerTrace(i.tenantID)
	maxLiveTraces := i.overrides.MaxLocalTracesPerUser(i.tenantID)

	// For each pre-marshalled trace, we need to unmarshal it and push to live traces
	for j, traceBytes := range req.Traces {
		traceID := req.Ids[j]
		// measure received bytes as sum of slice lengths
		// type byte is guaranteed to be 1 byte in size
		// ref: https://golang.org/ref/spec#Size_and_alignment_guarantees
		i.bytesReceivedTotal.WithLabelValues(i.tenantID, traceDataType).Add(float64(len(traceBytes.Slice)))

		// Unmarshal the trace
		trace := &tempopb.Trace{}
		if err := trace.Unmarshal(traceBytes.Slice); err != nil {
			level.Error(i.logger).Log("msg", "failed to unmarshal trace", "err", err)
			continue
		}

		// Reuse the byte slice now that we've unmarshalled it
		tempopb.ReuseByteSlices([][]byte{traceBytes.Slice})

		// test max trace size. use trace sizes over liveTraces b/c it tracks large traces across multiple flushes
		if maxBytes > 0 {
			traceSz := trace.Size()
			allowResult := i.traceSizes.Allow(traceID, traceSz, maxBytes)
			if !allowResult.IsAllowed {
				i.maxTraceLogger.Log("msg", overrides.ErrorPrefixTraceTooLarge, "max", maxBytes, "traceSz", traceSz, "totalSize", allowResult.CurrentTotalSize, "trace", hex.EncodeToString(traceID), "insight", true)
				overrides.RecordDiscardedSpans(countSpans(trace), overrides.ReasonTraceTooLarge, i.tenantID)
				continue
			}
		}

		i.liveTracesMtx.Lock()
		// Push each batch in the trace to live traces
		for _, batch := range trace.ResourceSpans {
			if len(batch.ScopeSpans) == 0 || len(batch.ScopeSpans[0].Spans) == 0 {
				continue
			}

			// Push to live traces with tenant-specific limits
			if err := i.liveTraces.PushWithTimestampAndLimits(ts, traceID, batch, uint64(maxLiveTraces), 0); err != nil {
				var reason string
				switch {
				case errors.Is(err, livetraces.ErrMaxLiveTracesExceeded):
					reason = overrides.ReasonLiveTracesExceeded
				case errors.Is(err, livetraces.ErrMaxTraceSizeExceeded): // this should technically never happen b/c we are passing 0 as max trace sz
					reason = overrides.ReasonTraceTooLarge
				default:
					reason = overrides.ReasonUnknown
				}
				level.Debug(i.logger).Log("msg", "dropped spans due to limits", "tenant", i.tenantID, "reason", reason)
				overrides.RecordDiscardedSpans(countSpans(trace), reason, i.tenantID)
				continue
			}
		}
		i.liveTracesMtx.Unlock()
	}
}

func countSpans(trace *tempopb.Trace) int {
	count := 0
	for _, b := range trace.ResourceSpans {
		for _, ss := range b.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	return count
}

func (i *instance) cutIdleTraces(immediate bool) error {
	i.liveTracesMtx.Lock()
	// Set metrics before cutting (similar to ingester)
	metricLiveTraces.WithLabelValues(i.tenantID).Set(float64(i.liveTraces.Len()))
	metricLiveTraceBytes.WithLabelValues(i.tenantID).Set(float64(i.liveTraces.Size()))

	tracesToCut := i.liveTraces.CutIdle(time.Now(), immediate)
	i.liveTracesMtx.Unlock()

	if len(tracesToCut) == 0 {
		return nil
	}

	// Sort by ID
	sort.Slice(tracesToCut, func(i, j int) bool {
		return bytes.Compare(tracesToCut[i].ID, tracesToCut[j].ID) == -1
	})

	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	for _, t := range tracesToCut {
		tr := &tempopb.Trace{
			ResourceSpans: t.Batches,
		}

		// Get trace timestamp bounds
		var start, end uint64
		for _, batch := range tr.ResourceSpans {
			for _, ss := range batch.ScopeSpans {
				for _, s := range ss.Spans {
					if start == 0 || s.StartTimeUnixNano < start {
						start = s.StartTimeUnixNano
					}
					if s.EndTimeUnixNano > end {
						end = s.EndTimeUnixNano
					}
				}
			}
		}

		// Convert from unix nanos to unix seconds
		startSeconds := uint32(start / uint64(time.Second))
		endSeconds := uint32(end / uint64(time.Second))

		// Constrain start/end with ingestion slack calculated off of liveTrace.createdAt and lastAppend.
		// createdAt and lastAppend are set via the record.Timestamp from kafka so they are "time.Now()" for the
		// ingestion of this trace.
		slackDuration := i.Cfg.WAL.IngestionSlack
		minStart := uint32(t.CreatedAt.Add(-slackDuration).Unix())
		maxEnd := uint32(t.LastAppend.Add(slackDuration).Unix())

		if startSeconds < minStart {
			startSeconds = minStart
		}
		if endSeconds > maxEnd {
			endSeconds = maxEnd
		}

		i.traceBuffer = append(i.traceBuffer, &bufferedTrace{
			ID:        t.ID,
			Trace:     tr,
			StartSecs: startSeconds,
			EndSecs:   endSeconds,
		})
		i.traceBufferBytes += uint64(tr.Size())

		// Track when the buffer started filling for MaxBlockDuration check
		if len(i.traceBuffer) == 1 {
			i.traceBufferStart = time.Now()
		}

		i.tracesCreatedTotal.Inc()
	}

	return nil
}

func (i *instance) getDedicatedColumns() backend.DedicatedColumns {
	if cols := i.overrides.DedicatedColumns(i.tenantID); cols != nil {
		_, err := cols.Validate()
		if err != nil {
			level.Error(i.logger).Log("msg", "Unable to apply overrides for dedicated attribute columns. Columns invalid.", "error", err)
			return i.Cfg.BlockConfig.DedicatedColumns
		}
		return cols
	}
	return i.Cfg.BlockConfig.DedicatedColumns
}

// cutBlocks checks if the trace buffer should be flushed to a complete block.
// When triggered it swaps out the buffer and returns it; the caller is responsible for
// driving the completion. Returns nil when nothing should be cut yet.
func (i *instance) cutBlocks(immediate bool) []*bufferedTrace {
	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	if len(i.traceBuffer) == 0 {
		return nil
	}

	if !immediate && time.Since(i.traceBufferStart) < i.Cfg.MaxBlockDuration && i.traceBufferBytes < i.Cfg.MaxBlockBytes {
		return nil
	}

	i.traceSizes.ClearIdle(i.traceBufferStart)

	// Swap out the buffer
	traces := i.traceBuffer
	bufferBytes := i.traceBufferBytes

	i.traceBuffer = nil
	i.traceBufferBytes = 0
	i.traceBufferStart = time.Time{}

	level.Info(i.logger).Log("msg", "queueing trace buffer for completion", "traces", len(traces), "bytes", bufferBytes)

	return traces
}

func (i *instance) completeBlock(ctx context.Context, traces []*bufferedTrace) (uuid.UUID, error) {
	if len(traces) == 0 {
		return uuid.Nil, nil
	}

	// Compute block time bounds from buffered traces
	var minStart, maxEnd uint32
	var totalBytes uint64
	for _, t := range traces {
		if minStart == 0 || t.StartSecs < minStart {
			minStart = t.StartSecs
		}
		if t.EndSecs > maxEnd {
			maxEnd = t.EndSecs
		}
		totalBytes += uint64(t.Trace.Size())
	}

	blockID := backend.NewUUID()

	ctx, span := tracer.Start(ctx, "instance.completeBlock",
		oteltrace.WithAttributes(
			attribute.String("tenant", i.tenantID),
			attribute.String("blockID", blockID.String()),
			attribute.Int("traces", len(traces)),
		))
	defer span.End()

	metricCompletionSize.Observe(float64(totalBytes))
	span.SetAttributes(attribute.Int64("total_bytes", int64(totalBytes)))

	dedicatedColumns := i.getDedicatedColumns()
	meta := &backend.BlockMeta{
		BlockID:           blockID,
		TenantID:          i.tenantID,
		DedicatedColumns:  dedicatedColumns,
		ReplicationFactor: backend.LiveStoreReplicationFactor,
		StartTime:         time.Unix(int64(minStart), 0),
		EndTime:           time.Unix(int64(maxEnd), 0),
	}

	reader := backend.NewReader(i.localBackend)
	writer := backend.NewWriter(i.localBackend)

	iter := newTraceBufferIter(traces)
	defer iter.Close()

	newMeta, err := i.completeBlockEncoding.CreateBlock(ctx, &i.Cfg.BlockConfig, meta, iter, reader, writer)
	if err != nil {
		level.Error(i.logger).Log("msg", "failed to create complete block", "blockID", blockID, "err", err)
		span.RecordError(err)
		return uuid.Nil, err
	}

	newBlock, err := i.completeBlockEncoding.OpenBlock(newMeta, reader)
	if err != nil {
		level.Error(i.logger).Log("msg", "failed to open complete block", "blockID", blockID, "err", err)
		span.RecordError(err)
		return uuid.Nil, err
	}

	id := (uuid.UUID)(newMeta.BlockID)

	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	i.completeBlocks[id] = ingester.NewLocalBlock(ctx, newBlock, i.localBackend)

	level.Info(i.logger).Log("msg", "completed block", "id", id.String())
	span.AddEvent("block completed successfully")
	return id, nil
}

func (i *instance) deleteOldBlocks() error {
	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	cutoff := time.Now().Add(-i.Cfg.CompleteBlockTimeout) // Delete blocks older than Complete Block Timeout

	for id, completeBlock := range i.completeBlocks {
		if completeBlock.BlockMeta().EndTime.Before(cutoff) {
			level.Info(i.logger).Log("msg", "deleting complete block", "block", id.String())
			err := i.localBackend.ClearBlock(id, i.tenantID)
			if err != nil {
				return err
			}
			delete(i.completeBlocks, id)
			metricBlocksClearedTotal.WithLabelValues("complete").Inc()
		}
	}

	return nil
}
