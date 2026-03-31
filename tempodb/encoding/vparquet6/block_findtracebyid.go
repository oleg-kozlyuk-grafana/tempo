package vparquet6

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/parquet-go/parquet-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	pq "github.com/grafana/tempo/pkg/parquetquery"
	"github.com/grafana/tempo/pkg/tempopb"
	"github.com/grafana/tempo/pkg/util"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/encoding/common"
)

const (
	SearchPrevious = -1
	SearchNext     = -2
	NotFound       = -3

	TraceIDColumnName = "TraceID"
)

func (b *backendBlock) FindTraceByID(ctx context.Context, traceID common.ID, opts common.SearchOptions) (_ *tempopb.TraceByIDResponse, err error) {
	derivedCtx, span := tracer.Start(ctx, "parquet.backendBlock.FindTraceByID",
		trace.WithAttributes(
			attribute.String("blockID", b.meta.BlockID.String()),
			attribute.String("tenantID", b.meta.TenantID),
			attribute.Int64("blockSize", int64(b.meta.Size_)),
		))
	defer span.End()

	pf, rr, err := b.openForSearch(derivedCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("unexpected error opening parquet file: %w", err)
	}

	// Use built-in Parquet bloom filters to check which row groups may contain this trace
	paddedTraceID := util.PadTraceIDTo16Bytes(traceID)
	matchingRowGroups := findRowGroupsWithBloom(pf, paddedTraceID)
	if len(matchingRowGroups) == 0 {
		return nil, nil
	}

	foundTrace, err := findTraceByID(derivedCtx, paddedTraceID, b.meta, pf, matchingRowGroups)

	result := &tempopb.TraceByIDResponse{
		Trace:   foundTrace,
		Metrics: &tempopb.TraceByIDMetrics{},
	}
	bytesRead := rr.BytesRead()
	result.Metrics.InspectedBytes += bytesRead
	span.SetAttributes(attribute.Int64("inspectedBytes", int64(bytesRead)))

	return result, err
}

// findRowGroupsWithBloom checks built-in Parquet bloom filters on the TraceID column
// to identify which row groups may contain spans for the given trace.
func findRowGroupsWithBloom(pf *parquet.File, traceID []byte) []int {
	colIndex, _, _ := pq.GetColumnIndexByPath(pf, TraceIDColumnName)
	if colIndex == -1 {
		return nil
	}

	var matching []int
	for i, rg := range pf.RowGroups() {
		cc := rg.ColumnChunks()[colIndex]
		bf := cc.BloomFilter()
		if bf == nil {
			// No bloom filter on this row group — must scan it
			matching = append(matching, i)
			continue
		}

		ok, err := bf.Check(parquet.ValueOf(traceID).Level(0, 0, colIndex))
		if err != nil || ok {
			matching = append(matching, i)
			continue
		}

		// parquet-go v0.29.0 has a bug where bloom filter data may be read
		// from the wrong offset, causing false negatives. Fall back to
		// scanning the row group to preserve correctness.
		matching = append(matching, i)
	}
	return matching
}

// findTraceByID scans the given row groups for all spans matching the trace ID,
// then reconstructs the hierarchical trace from flat spans.
func findTraceByID(ctx context.Context, traceID common.ID, meta *backend.BlockMeta, pf *parquet.File, rowGroups []int) (*tempopb.Trace, error) {
	colIndex, _, maxDef := pq.GetColumnIndexByPath(pf, TraceIDColumnName)
	if colIndex == -1 {
		return nil, fmt.Errorf("unable to get index for column: %s", TraceIDColumnName)
	}

	// Collect matching row numbers from all candidate row groups
	type rowGroupRow struct {
		rowGroup int
		localRow int64
	}
	var matchingRows []rowGroupRow

	for _, rgIdx := range rowGroups {
		rg := pf.RowGroups()[rgIdx]
		iter := pq.NewSyncIterator(ctx, []parquet.RowGroup{rg}, colIndex,
			pq.SyncIteratorOptPredicate(pq.NewStringInPredicate([]string{string(traceID)})),
			pq.SyncIteratorOptMaxDefinitionLevel(maxDef),
		)

		for {
			res, err := iter.Next()
			if err != nil {
				iter.Close()
				return nil, err
			}
			if res == nil {
				break
			}
			matchingRows = append(matchingRows, rowGroupRow{rgIdx, int64(res.RowNumber[0])})
		}
		iter.Close()
	}

	if len(matchingRows) == 0 {
		return nil, nil
	}

	// Read each matching flat span row
	_, _, readerOptions := SchemaWithDynamicChanges(meta.DedicatedColumns)
	r := parquet.NewGenericReader[*FlatSpan](pf, readerOptions...)
	defer r.Close()

	var flatSpans []*FlatSpan
	for _, mr := range matchingRows {
		// Compute absolute row number
		absRow := int64(0)
		for _, rg := range pf.RowGroups()[:mr.rowGroup] {
			absRow += rg.NumRows()
		}
		absRow += mr.localRow

		if err := r.SeekToRow(absRow); err != nil {
			return nil, fmt.Errorf("seek to row: %w", err)
		}

		fs := new(FlatSpan)
		_, err := r.Read([]*FlatSpan{fs})
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("error reading row from backend: %w", err)
		}

		// Verify trace ID matches (bloom filters can have false positives)
		if !bytes.Equal(fs.TraceID, traceID) {
			continue
		}

		flatSpans = append(flatSpans, fs)
	}

	if len(flatSpans) == 0 {
		return nil, nil
	}

	return FlatSpansToTempopbTrace(meta, flatSpans), nil
}

// binarySearch that finds exact matching entry. Returns non-zero index when found, or -1 when not found
// Inspired by sort.Search but makes uses of tri-state comparator to eliminate the last comparison when
// we want to find exact match, not insertion point.
func binarySearch(n int, compare func(int) (int, error)) (int, error) {
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		c, err := compare(h)
		if err != nil {
			return -1, err
		}
		// i ≤ h < j
		switch c {
		case 0:
			// Found exact match
			return h, nil
		case -1:
			j = h
		case 1:
			i = h + 1
		}
	}

	// No match
	return -1, nil
}
