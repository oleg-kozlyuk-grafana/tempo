package vparquet6

import (
	"bytes"
	"context"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	tempo_io "github.com/grafana/tempo/pkg/io"
	"github.com/grafana/tempo/pkg/util/test"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/backend/local"
	"github.com/grafana/tempo/tempodb/encoding/common"
)

func TestBackendBlockFindTraceByID(t *testing.T) {
	rawR, rawW, _, err := local.New(&local.Config{
		Path: t.TempDir(),
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	w := backend.NewWriter(rawW)
	ctx := context.Background()

	cfg := &common.BlockConfig{
		BloomFP:             0.01,
		BloomShardSizeBytes: 100 * 1024,
	}

	// Test data - sorted by trace ID
	// Find trace by ID uses the column and page bounds,
	// which by default only stores 16 bytes, which is the first
	// half of the trace ID (which is stored as 32 hex text)
	// Therefore it is important that the test data here has
	// full-length trace IDs.
	type traceData struct {
		id    []byte
		spans []FlatSpan
	}
	var traces []traceData
	for i := 0; i < 16; i++ {
		id := test.ValidTraceID(nil)
		bar := "bar"
		spans := []FlatSpan{
			{
				TraceID:             id,
				ResourceServiceName: "s",
				SpanID:              []byte{},
				ParentSpanID:        []byte{},
				Name:                "hello",
				Attrs: []Attribute{
					attr("foo", bar),
				},
			},
		}
		traces = append(traces, traceData{id: id, spans: spans})
	}

	// Sort
	sort.Slice(traces, func(i, j int) bool {
		return bytes.Compare(traces[i].id, traces[j].id) == -1
	})

	meta := backend.NewBlockMeta("fake", uuid.New(), VersionString)
	meta.TotalObjects = int64(len(traces))
	s, newMeta := newStreamingBlock(ctx, cfg, meta, r, w, tempo_io.NewBufferedWriter)

	// Write test data, occasionally flushing (cutting new row group)
	rowGroupSize := 5
	for _, tr := range traces {
		err := s.Add(tr.id, tr.spans, 0, 0)
		require.NoError(t, err)
		if s.CurrentBufferedObjects() >= rowGroupSize {
			_, err = s.Flush()
			require.NoError(t, err)
		}
	}
	_, err = s.Complete()
	require.NoError(t, err)

	b := newBackendBlock(newMeta, r)

	// Now find and verify all test traces
	for _, tr := range traces {
		ptrs := make([]*FlatSpan, len(tr.spans))
		for i := range tr.spans {
			ptrs[i] = &tr.spans[i]
		}
		wantProto := FlatSpansToTempopbTrace(newMeta, ptrs)

		gotProto, err := b.FindTraceByID(ctx, tr.id, common.DefaultSearchOptions())
		require.NoError(t, err)
		require.NotNil(t, gotProto, "trace not found for id %x", tr.id)
		require.Equal(t, wantProto, gotProto.Trace)
	}
}

func TestBackendBlockFindTraceByID_TestData(t *testing.T) {
	// test-data was generated with the old hierarchical schema and is
	// incompatible with the flat (one-row-per-span) format.
	t.Skip("test-data needs regeneration for flat schema")

	rawR, _, _, err := local.New(&local.Config{
		Path: "./test-data",
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	ctx := context.Background()

	blocks, _, err := r.Blocks(ctx, "single-tenant")
	require.NoError(t, err)

	if len(blocks) == 0 {
		t.Skip("no test data blocks found")
	}

	meta, err := r.BlockMeta(ctx, blocks[0], "single-tenant")
	require.NoError(t, err)

	b := newBackendBlock(meta, r)

	iter, err := b.rawIter(context.Background(), newRowPool(10))
	require.NoError(t, err)

	for {
		id, row, err := iter.Next(context.Background())
		require.NoError(t, err)

		if row == nil {
			break
		}

		protoTr, err := b.FindTraceByID(ctx, id, common.DefaultSearchOptions())
		require.NoError(t, err)
		require.NotNil(t, protoTr)
	}
}

func TestBackendBlockTraceRoundtrip(t *testing.T) {
	testCases := []struct {
		name string
		tr   []FlatSpan
		dc   backend.DedicatedColumns
	}{
		{
			name: "fullypopulated",
			tr:   fullyPopulatedTestTrace(test.ValidTraceID(nil)),
			dc:   test.MakeDedicatedColumns(),
		},
		{
			name: "mixed array/non-array",
			tr:   func() []FlatSpan { tr, _ := mixedArrayTestTrace(); return tr }(),
			dc:   func() backend.DedicatedColumns { _, dc := mixedArrayTestTrace(); return dc }(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx   = t.Context()
				block = makeBackendBlockWithTracesWithDedicatedColumns(t, [][]FlatSpan{tc.tr}, tc.dc)
			)
			ptrs := make([]*FlatSpan, len(tc.tr))
			for i := range tc.tr {
				ptrs[i] = &tc.tr[i]
			}
			wantProto := FlatSpansToTempopbTrace(block.meta, ptrs)

			gotProto, err := block.FindTraceByID(ctx, tc.tr[0].TraceID, common.DefaultSearchOptions())
			require.NoError(t, err)
			require.Equal(t, wantProto, gotProto.Trace)
		})
	}
}

func BenchmarkFindTraceByID(b *testing.B) {
	ctx := context.TODO()
	traceID := []byte{}
	block := blockForBenchmarks(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr, err := block.FindTraceByID(ctx, traceID, common.DefaultSearchOptions())
		require.NoError(b, err)
		require.NotNil(b, tr)
	}
}
