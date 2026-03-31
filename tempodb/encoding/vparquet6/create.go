package vparquet6

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/grafana/tempo/pkg/dataquality"
	tempo_io "github.com/grafana/tempo/pkg/io"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/encoding/common"
	"github.com/parquet-go/parquet-go"
)

type backendWriter struct {
	ctx      context.Context
	w        backend.Writer
	name     string
	blockID  uuid.UUID
	tenantID string
	tracker  backend.AppendTracker
}

var _ io.WriteCloser = (*backendWriter)(nil)

func (b *backendWriter) Write(p []byte) (n int, err error) {
	b.tracker, err = b.w.Append(b.ctx, b.name, b.blockID, b.tenantID, b.tracker, p)
	return len(p), err
}

func (b *backendWriter) Close() error {
	return b.w.CloseAppend(b.ctx, b.tracker)
}

func CreateBlock(ctx context.Context, cfg *common.BlockConfig, meta *backend.BlockMeta, i common.Iterator, r backend.Reader, to backend.Writer) (*backend.BlockMeta, error) {
	s, newMeta := newStreamingBlock(ctx, cfg, meta, r, to, tempo_io.NewBufferedWriter)

	var next func(context.Context) error

	if ii, ok := i.(*commonIterator); ok {
		next = func(ctx context.Context) error {
			// Use internal iterator and avoid translation to/from proto
			id, row, err := ii.NextRow(ctx)
			if err != nil {
				return err
			}
			if row == nil {
				return io.EOF
			}
			err = s.AddRaw(id, row, 0, 0) // start and end time of the wal meta are used.
			if err != nil {
				return err
			}

			completeBlockRowPool.Put(row)
			return nil
		}
	} else {
		// Need to convert from proto->flat spans
		var buffer []FlatSpan
		next = func(context.Context) error {
			id, tr, err := i.Next(ctx)
			if err != nil {
				return err
			}
			if tr == nil {
				return io.EOF
			}

			// Copy ID to allow it to escape the iterator.
			id = append([]byte(nil), id...)

			var connected bool
			buffer, connected = traceToParquet(newMeta, id, tr, buffer)
			if !connected {
				dataquality.WarnDisconnectedTrace(meta.TenantID, dataquality.PhaseTraceWalToComplete)
			}

			// Check for rootless trace
			hasRoot := false
			for idx := range buffer {
				if buffer[idx].IsRoot() {
					hasRoot = true
					break
				}
			}
			if !hasRoot {
				dataquality.WarnRootlessTrace(meta.TenantID, dataquality.PhaseTraceWalToComplete)
			}

			err = s.Add(id, buffer, 0, 0) // start and end time are set outside
			if err != nil {
				return err
			}

			return nil
		}
	}

	for {
		err := next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		if s.EstimatedBufferedBytes() > cfg.RowGroupSizeBytes {
			_, err = s.Flush()
			if err != nil {
				return nil, err
			}

		}
	}

	_, err := s.Complete()
	if err != nil {
		return nil, err
	}

	return newMeta, nil
}

type streamingBlock struct {
	ctx  context.Context
	meta *backend.BlockMeta
	bw   tempo_io.BufferedWriteFlusher
	pw   *parquet.GenericWriter[*FlatSpan]
	w    *backendWriter
	r    backend.Reader
	to   backend.Writer

	withNoCompactFlag bool

	currentBufferedTraces int
	currentBufferedBytes  int
}

func newStreamingBlock(ctx context.Context, cfg *common.BlockConfig, meta *backend.BlockMeta, r backend.Reader, to backend.Writer, createBufferedWriter func(w io.Writer) tempo_io.BufferedWriteFlusher) (*streamingBlock, *backend.BlockMeta) {
	newMeta := backend.NewBlockMeta(meta.TenantID, (uuid.UUID)(meta.BlockID), VersionString)
	newMeta.StartTime = meta.StartTime
	newMeta.EndTime = meta.EndTime
	newMeta.ReplicationFactor = meta.ReplicationFactor
	newMeta.DedicatedColumns = filterDedicatedColumns(meta.DedicatedColumns)

	var (
		w                   = &backendWriter{ctx, to, DataFileName, (uuid.UUID)(meta.BlockID), meta.TenantID, nil}
		bw                  = createBufferedWriter(w)
		_, writerOptions, _ = SchemaWithDynamicChanges(meta.DedicatedColumns)
		pw                  = parquet.NewGenericWriter[*FlatSpan](bw, writerOptions...)
	)

	return &streamingBlock{
		ctx:               ctx,
		meta:              newMeta,
		bw:                bw,
		pw:                pw,
		w:                 w,
		r:                 r,
		to:                to,
		withNoCompactFlag: cfg.CreateWithNoCompactFlag,
	}, newMeta
}

// Add writes a set of flat spans (one trace) to the block.
func (b *streamingBlock) Add(id []byte, spans []FlatSpan, start, end uint32) error {
	ptrs := make([]*FlatSpan, len(spans))
	for i := range spans {
		ptrs[i] = &spans[i]
	}
	_, err := b.pw.Write(ptrs)
	if err != nil {
		return err
	}

	b.meta.ObjectAdded(start, end)
	b.currentBufferedTraces++
	b.currentBufferedBytes += estimateMarshalledSizeFromFlatSpans(spans)

	return nil
}

func (b *streamingBlock) AddRaw(id []byte, row parquet.Row, start, end uint32) error {
	_, err := b.pw.WriteRows([]parquet.Row{row})
	if err != nil {
		return err
	}

	b.meta.ObjectAdded(start, end)
	b.currentBufferedTraces++
	b.currentBufferedBytes += estimateMarshalledSizeFromParquetRow(row)

	return nil
}

func (b *streamingBlock) EstimatedBufferedBytes() int {
	return b.currentBufferedBytes
}

func (b *streamingBlock) CurrentBufferedObjects() int {
	return b.currentBufferedTraces
}

func (b *streamingBlock) Flush() (int, error) {
	// Flush row group
	err := b.pw.Flush()
	if err != nil {
		return 0, err
	}

	n := b.bw.Len()
	b.meta.Size_ += uint64(n)
	b.meta.TotalRecords++
	b.currentBufferedTraces = 0
	b.currentBufferedBytes = 0

	// Flush to underlying writer
	return n, b.bw.Flush()
}

func (b *streamingBlock) Complete() (int, error) {
	// Flush final row group
	b.meta.TotalRecords++
	err := b.pw.Flush()
	if err != nil {
		return 0, err
	}

	// Close parquet file. This writes the footer and metadata.
	err = b.pw.Close()
	if err != nil {
		return 0, err
	}

	// Now Flush and close out in-memory buffer
	n := b.bw.Len()
	b.meta.Size_ += uint64(n)
	err = b.bw.Flush()
	if err != nil {
		return 0, err
	}

	err = b.bw.Close()
	if err != nil {
		return 0, err
	}

	err = b.w.Close()
	if err != nil {
		return 0, err
	}

	// Read the footer size out of the parquet footer
	buf := make([]byte, 8)
	err = b.r.ReadRange(b.ctx, DataFileName, (uuid.UUID)(b.meta.BlockID), b.meta.TenantID, b.meta.Size_-8, buf, nil)
	if err != nil {
		return 0, fmt.Errorf("error reading parquet file footer: %w", err)
	}
	if string(buf[4:8]) != "PAR1" {
		return 0, errors.New("failed to confirm magic footer while writing a new parquet block")
	}
	b.meta.FooterSize = binary.LittleEndian.Uint32(buf[0:4])

	if b.withNoCompactFlag {
		err := b.to.WriteNoCompactFlag(b.ctx, (uuid.UUID)(b.meta.BlockID), b.meta.TenantID)
		if err != nil {
			return 0, fmt.Errorf("unexpected error writing nocompact flag: %w", err)
		}
	}

	return n, writeBlockMeta(b.ctx, b.to, b.meta)
}

// estimateMarshalledSizeFromFlatSpans estimates the size of flat spans when written to parquet.
func estimateMarshalledSizeFromFlatSpans(spans []FlatSpan) (size int) {
	for i := range spans {
		size += estimateMarshalledSizeFromFlatSpan(&spans[i])
	}
	return
}

func estimateMarshalledSizeFromFlatSpan(s *FlatSpan) (size int) {
	// Trace-level fields
	size += len(s.TraceID) + len(s.TraceIDText) + 8*3 + len(s.RootServiceName) + len(s.RootSpanName)

	// Resource-level fields
	size += len(s.ResourceServiceName) + 4
	size += estimateAttrSize(s.ResourceAttrs)

	// Scope-level fields
	size += len(s.ScopeName) + len(s.ScopeVersion) + 4
	size += estimateAttrSize(s.ScopeAttrs)

	// Span-level fields
	size += len(s.SpanID) + len(s.ParentSpanID)
	size += len(s.Name) + len(s.StatusMessage) + len(s.TraceState)
	size += 8 * 2 // StartTimeUnixNano, DurationNano
	size += 4 * 8 // Kind, StatusCode, ParentID, NestedSetLeft, NestedSetRight, ChildCount, DroppedAttributesCount, DroppedEventsCount, DroppedLinksCount, rounded times
	size += estimateAttrSize(s.Attrs)
	size += estimateEventsSize(s.Events)
	size += estimateLinksSize(s.Links)

	return
}

func estimateAttrSize(attrs []Attribute) (size int) {
	size += len(attrs) * 7 // 7 attribute lvl fields

	for _, a := range attrs {
		size += max(0, len(a.Value)-1)
		size += max(0, len(a.ValueInt)-1)
		size += max(0, len(a.ValueDouble)-1)
		size += max(0, len(a.ValueBool)-1)
	}

	return
}

func estimateEventsSize(events []Event) (size int) {
	for _, e := range events {
		size += 4  // 4 event lvl fields
		size += 10 // 10 dedicated columns
		size += estimateAttrSize(e.Attrs)
	}
	return
}

func estimateLinksSize(links []Link) (size int) {
	for _, l := range links {
		size += 5 // 5 link lvl fields
		size += len(l.TraceID) + len(l.SpanID)
		size += estimateAttrSize(l.Attrs)
	}
	return
}
