package vparquet6

import (
	"testing"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/tempo/pkg/util/test"
	"github.com/grafana/tempo/tempodb/backend"
)

func TestCombiner(t *testing.T) {
	methods := []func(a, b []FlatSpan) ([]FlatSpan, int, bool){
		func(a, b []FlatSpan) ([]FlatSpan, int, bool) {
			c := NewCombiner()
			c.Consume(a)
			c.Consume(b)
			return c.Result()
		},
		func(a, b []FlatSpan) ([]FlatSpan, int, bool) {
			c := NewCombiner()
			c.Consume(a)
			c.ConsumeWithFinal(b, true)
			return c.Result()
		},
	}

	tests := []struct {
		name          string
		traceA        []FlatSpan
		traceB        []FlatSpan
		expectedTotal int
		expectedTrace []FlatSpan
	}{
		{
			name:          "nil traceA",
			traceA:        nil,
			traceB:        []FlatSpan{},
			expectedTotal: -1,
		},
		{
			name:          "nil traceB",
			traceA:        []FlatSpan{},
			traceB:        nil,
			expectedTotal: -1,
		},
		{
			name:          "empty traces",
			traceA:        []FlatSpan{},
			traceB:        []FlatSpan{},
			expectedTotal: -1,
		},
		{
			name: "root meta from second overrides empty first",
			traceA: []FlatSpan{
				{TraceID: []byte{0x00, 0x01}},
			},
			traceB: []FlatSpan{
				{
					TraceID:                []byte{0x00, 0x01},
					RootServiceName:        "serviceNameB",
					RootSpanName:           "spanNameB",
					TraceStartTimeUnixNano: 10,
					TraceEndTimeUnixNano:   20,
					TraceDurationNano:      10,
					// Use a different SpanID so it's not a dupe
					SpanID: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
				},
			},
			expectedTotal: 2,
			expectedTrace: []FlatSpan{
				{
					TraceID:                []byte{0x00, 0x01},
					RootServiceName:        "serviceNameB",
					RootSpanName:           "spanNameB",
					TraceStartTimeUnixNano: 10,
					TraceEndTimeUnixNano:   20,
				},
				{
					TraceID:                []byte{0x00, 0x01},
					RootServiceName:        "serviceNameB",
					RootSpanName:           "spanNameB",
					TraceStartTimeUnixNano: 10,
					TraceEndTimeUnixNano:   20,
					SpanID:                 []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
				},
			},
		},
		{
			name: "if both set first root name wins",
			traceA: []FlatSpan{
				{
					TraceID:         []byte{0x00, 0x01},
					RootServiceName: "serviceNameA",
					RootSpanName:    "spanNameA",
				},
			},
			traceB: []FlatSpan{
				{
					TraceID:         []byte{0x00, 0x01},
					RootServiceName: "serviceNameB",
					RootSpanName:    "spanNameB",
					SpanID:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
				},
			},
			expectedTotal: 2,
			expectedTrace: []FlatSpan{
				{
					TraceID:         []byte{0x00, 0x01},
					RootServiceName: "serviceNameA",
					RootSpanName:    "spanNameA",
				},
				{
					TraceID:         []byte{0x00, 0x01},
					RootServiceName: "serviceNameA",
					RootSpanName:    "spanNameA",
					SpanID:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
				},
			},
		},
		{
			name: "combine spans from different services",
			traceA: []FlatSpan{
				{
					TraceID:             []byte{0x00, 0x01},
					RootServiceName:     "serviceNameA",
					ResourceServiceName: "serviceNameA",
					SpanID:              []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
					NestedSetLeft:       1,
					NestedSetRight:      2,
				},
			},
			traceB: []FlatSpan{
				{
					TraceID:             []byte{0x00, 0x01},
					RootServiceName:     "serviceNameB",
					ResourceServiceName: "serviceNameB",
					SpanID:              []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
					ParentSpanID:        []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
				},
			},
			expectedTotal: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, m := range methods {
				actualTrace, actualTotal, _ := m(tt.traceA, tt.traceB)
				assert.Equal(t, tt.expectedTotal, actualTotal)
				if tt.expectedTrace != nil {
					// Compare only the fields we care about (root service, root span, timing)
					assert.Equal(t, len(tt.expectedTrace), len(actualTrace))
					for i := range tt.expectedTrace {
						if i >= len(actualTrace) {
							break
						}
						assert.Equal(t, tt.expectedTrace[i].RootServiceName, actualTrace[i].RootServiceName)
						assert.Equal(t, tt.expectedTrace[i].RootSpanName, actualTrace[i].RootSpanName)
						assert.Equal(t, tt.expectedTrace[i].TraceStartTimeUnixNano, actualTrace[i].TraceStartTimeUnixNano)
						assert.Equal(t, tt.expectedTrace[i].TraceEndTimeUnixNano, actualTrace[i].TraceEndTimeUnixNano)
					}
				}
			}
		})
	}
}

func TestCombinerReturnsDuplicates(t *testing.T) {
	tests := []struct {
		name          string
		traceA        []FlatSpan
		traceB        []FlatSpan
		expectedDupes int
	}{
		{
			name:          "nil traceA",
			traceA:        nil,
			traceB:        []FlatSpan{},
			expectedDupes: 0,
		},
		{
			name:          "nil traceB",
			traceA:        []FlatSpan{},
			traceB:        nil,
			expectedDupes: 0,
		},
		{
			name:          "empty traces",
			traceA:        []FlatSpan{},
			traceB:        []FlatSpan{},
			expectedDupes: 0,
		},
		{
			name: "no dupes",
			traceA: []FlatSpan{
				{
					TraceID:             []byte{0x00, 0x01},
					RootServiceName:     "serviceNameA",
					ResourceServiceName: "serviceNameA",
					SpanID:              []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
					NestedSetLeft:       1,
					NestedSetRight:      2,
				},
			},
			traceB: []FlatSpan{
				{
					TraceID:             []byte{0x00, 0x01},
					RootServiceName:     "serviceNameB",
					ResourceServiceName: "serviceNameB",
					SpanID:              []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
					ParentSpanID:        []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
				},
			},
			expectedDupes: 0,
		},
		{
			name: "one dupe",
			traceA: []FlatSpan{
				{
					TraceID:             []byte{0x00, 0x01},
					RootServiceName:     "serviceNameA",
					ResourceServiceName: "serviceNameA",
					SpanID:              []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
					NestedSetLeft:       1,
					NestedSetRight:      2,
				},
			},
			traceB: []FlatSpan{
				{
					TraceID:             []byte{0x00, 0x01},
					RootServiceName:     "serviceNameB",
					ResourceServiceName: "serviceNameB",
					SpanID:              []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
				},
				{
					TraceID:             []byte{0x00, 0x01},
					RootServiceName:     "serviceNameB",
					ResourceServiceName: "serviceNameB",
					SpanID:              []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03},
					ParentSpanID:        []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
					StatusCode:          2,
				},
			},
			expectedDupes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmb := NewCombiner()

			cmb.Consume(tt.traceA)
			actualDupes := cmb.Consume(tt.traceB)

			assert.Equal(t, tt.expectedDupes, actualDupes)
		})
	}
}

func BenchmarkCombine(b *testing.B) {
	batchCount := 100
	spanCounts := []int{
		100, 1000, 10000,
	}

	for _, spanCount := range spanCounts {
		b.Run("SpanCount:"+humanize.SI(float64(batchCount*spanCount), ""), func(b *testing.B) {
			id1 := test.ValidTraceID(nil)
			tr1, _ := traceToParquet(&backend.BlockMeta{}, id1, test.MakeTraceWithSpanCount(batchCount, spanCount, id1), nil)

			id2 := test.ValidTraceID(nil)
			tr2, _ := traceToParquet(&backend.BlockMeta{}, id2, test.MakeTraceWithSpanCount(batchCount, spanCount, id2), nil)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				c := NewCombiner()
				c.ConsumeWithFinal(tr1, false)
				c.ConsumeWithFinal(tr2, true)
				c.Result()
			}
		})
	}
}

func BenchmarkSortFlatSpans(b *testing.B) {
	batchCount := 100
	spanCounts := []int{
		100, 1000, 10000,
	}

	for _, spanCount := range spanCounts {
		b.Run("SpanCount:"+humanize.SI(float64(batchCount*spanCount), ""), func(b *testing.B) {
			id := test.ValidTraceID(nil)
			tr, _ := traceToParquet(&backend.BlockMeta{}, id, test.MakeTraceWithSpanCount(batchCount, spanCount, id), nil)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				SortFlatSpans(tr)
			}
		})
	}
}
