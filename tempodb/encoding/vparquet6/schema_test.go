package vparquet6

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/dustin/go-humanize"
	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/tempo/pkg/tempopb"
	v1 "github.com/grafana/tempo/pkg/tempopb/common/v1"
	v1_resource "github.com/grafana/tempo/pkg/tempopb/resource/v1"
	v1_trace "github.com/grafana/tempo/pkg/tempopb/trace/v1"
	"github.com/grafana/tempo/pkg/util/test"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/encoding/common"
)

func flatSpanPtrs(spans []FlatSpan) []*FlatSpan {
	ptrs := make([]*FlatSpan, len(spans))
	for i := range spans {
		ptrs[i] = &spans[i]
	}
	return ptrs
}

func TestProtoParquetRoundTrip(t *testing.T) {
	// This test round trips a proto trace and checks that the transformation works as expected
	// Proto -> Parquet -> Proto
	meta := backend.BlockMeta{
		DedicatedColumns: test.MakeDedicatedColumns(),
	}
	traceIDA := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}

	expectedTrace := FlatSpansToTempopbTrace(&meta, flatSpanPtrs(fullyPopulatedTestTrace(traceIDA)))

	parquetTrace, connected := traceToParquet(&meta, traceIDA, expectedTrace, nil)
	require.True(t, connected)
	actualTrace := FlatSpansToTempopbTrace(&meta, flatSpanPtrs(parquetTrace))

	tempopbTraceEqual(t, expectedTrace, actualTrace)
}

func TestProtoToParquetEmptyTrace(t *testing.T) {
	got, connected := traceToParquet(&backend.BlockMeta{}, nil, &tempopb.Trace{}, nil)
	require.False(t, connected)
	require.Empty(t, got)
}

func TestProtoParquetRando(t *testing.T) {
	var buffer []FlatSpan
	for i := 0; i < 100; i++ {
		batches := rand.Intn(15)
		id := test.ValidTraceID(nil)
		expectedTrace := test.AddDedicatedAttributes(test.MakeTrace(batches, id))

		parqTr, _ := traceToParquet(&backend.BlockMeta{}, id, expectedTrace, buffer)
		actualTrace := FlatSpansToTempopbTrace(&backend.BlockMeta{}, flatSpanPtrs(parqTr))
		require.Equal(t, expectedTrace, actualTrace)
	}
}

func TestFieldsAreCleared(t *testing.T) {
	meta := backend.BlockMeta{
		DedicatedColumns: test.MakeDedicatedColumns(),
	}

	traceID := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	complexTrace := FlatSpansToTempopbTrace(&meta, flatSpanPtrs(fullyPopulatedTestTrace(traceID)))
	simpleTrace := &tempopb.Trace{
		ResourceSpans: []*v1_trace.ResourceSpans{
			{
				Resource: &v1_resource.Resource{
					Attributes: []*v1.KeyValue{
						{Key: LabelServiceName, Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "service1"}}},
						{Key: "i", Value: &v1.AnyValue{Value: &v1.AnyValue_DoubleValue{DoubleValue: 123.456}}},
					},
				},
				ScopeSpans: []*v1_trace.ScopeSpans{
					{
						Scope: &v1.InstrumentationScope{},
						Spans: []*v1_trace.Span{
							{
								TraceId: traceID,
								Status: &v1_trace.Status{
									Code: v1_trace.Status_STATUS_CODE_ERROR,
								},
								Attributes: []*v1.KeyValue{
									{Key: "a", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 11}}},
									{Key: "b", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "bbb"}}},
									{Key: "c", Value: &v1.AnyValue{Value: &v1.AnyValue_BoolValue{BoolValue: true}}},
									{Key: "d", Value: &v1.AnyValue{Value: &v1.AnyValue_DoubleValue{DoubleValue: 111.11}}},
								},
								Events: []*v1_trace.Span_Event{
									{
										Attributes: []*v1.KeyValue{
											{Key: "event-attr", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 123}}},
										},
									},
								},
								Links: []*v1_trace.Span_Link{
									{
										Attributes: []*v1.KeyValue{
											{Key: "link-attr", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 123}}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	expected := []FlatSpan{
		{
			TraceID:             traceID,
			TraceIDText:         "102030405060708090a0b0c0d0e0f",
			RootServiceName:     "service1",
			ResourceServiceName: "service1",
			ResourceAttrs: []Attribute{
				attr("i", 123.456),
			},
			ServiceStats:   []ServiceStats{{ServiceName: "service1", SpanCount: 1, ErrorCount: 1}},
			StatusCode:     2,
			ParentID:       -1,
			NestedSetLeft:  1,
			NestedSetRight: 2,
			Attrs: []Attribute{
				attr("a", 11),
				attr("b", "bbb"),
				attr("c", true),
				attr("d", 111.11),
			},
			Events: []Event{{
				Attrs: []Attribute{
					attr("event-attr", 123),
				},
			}},
			Links: []Link{{
				Attrs: []Attribute{
					attr("link-attr", 123),
				},
			}},
		},
	}

	// first convert a trace that sets all fields and then convert
	// a minimal trace to make sure nothing bleeds through
	var buffer []FlatSpan
	buffer, _ = traceToParquet(&meta, traceID, complexTrace, buffer)
	actualTrace, _ := traceToParquet(&meta, traceID, simpleTrace, buffer)

	flatSpansEqual(t, expected, actualTrace)
}

func TestTraceToParquet(t *testing.T) {
	meta := backend.BlockMeta{DedicatedColumns: test.MakeDedicatedColumns()}
	traceID := common.ID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}

	tsc := []struct {
		name     string
		id       common.ID
		trace    tempopb.Trace
		expected []FlatSpan
	}{
		{
			name: "span scope and resource attributes",
			id:   traceID,
			trace: tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{{
					Resource: &v1_resource.Resource{
						Attributes: []*v1.KeyValue{
							{Key: "res.attr", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 123}}},
							{Key: "service.name", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "service-a"}}},
							{Key: "cluster", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "cluster-a"}}},
							{Key: "namespace", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "namespace-a"}}},
							{Key: "pod", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "pod-a"}}},
							{Key: "container", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "container-a"}}},
							{Key: "k8s.cluster.name", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "k8s-cluster-a"}}},
							{Key: "k8s.namespace.name", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "k8s-namespace-a"}}},
							{Key: "k8s.pod.name", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "k8s-pod-a"}}},
							{Key: "k8s.container.name", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "k8s-container-a"}}},
							{Key: "dedicated.resource.1", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-resource-attr-value-1"}}},
							{Key: "dedicated.resource.2", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-resource-attr-value-2"}}},
							{Key: "dedicated.resource.3", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-resource-attr-value-3"}}},
							{Key: "dedicated.resource.4", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-resource-attr-value-4"}}},
							{Key: "dedicated.resource.5", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-resource-attr-value-5"}}},
							{Key: "res.string.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
								Values: []*v1.AnyValue{
									{Value: &v1.AnyValue_StringValue{StringValue: "one"}},
									{Value: &v1.AnyValue_StringValue{StringValue: "two"}},
									{Value: &v1.AnyValue_StringValue{StringValue: "three"}},
								},
							}}}},
							{Key: "res.int.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
								Values: []*v1.AnyValue{
									{Value: &v1.AnyValue_IntValue{IntValue: 1}},
									{Value: &v1.AnyValue_IntValue{IntValue: 2}},
								},
							}}}},
							{Key: "res.double.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
								Values: []*v1.AnyValue{
									{Value: &v1.AnyValue_DoubleValue{DoubleValue: 1.1}},
									{Value: &v1.AnyValue_DoubleValue{DoubleValue: 2.2}},
									{Value: &v1.AnyValue_DoubleValue{DoubleValue: 3.3}},
								},
							}}}},
							{Key: "res.bool.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
								Values: []*v1.AnyValue{
									{Value: &v1.AnyValue_BoolValue{BoolValue: true}},
									{Value: &v1.AnyValue_BoolValue{BoolValue: false}},
									{Value: &v1.AnyValue_BoolValue{BoolValue: true}},
									{Value: &v1.AnyValue_BoolValue{BoolValue: true}},
								},
							}}}},
						},
					},
					ScopeSpans: []*v1_trace.ScopeSpans{{
						Scope: &v1.InstrumentationScope{
							Name:                   "scope-a",
							Version:                "scope-a-version",
							DroppedAttributesCount: 101,
							Attributes: []*v1.KeyValue{
								{Key: "scope.attr.str", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "scope-val-1"}}},
								{Key: "scope.attr.int", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 102}}},
								{Key: "scope.attr.float", Value: &v1.AnyValue{Value: &v1.AnyValue_DoubleValue{DoubleValue: 1.234}}},
								{Key: "scope.attr.bool", Value: &v1.AnyValue{Value: &v1.AnyValue_BoolValue{BoolValue: true}}},
								{Key: "scope.string.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
									Values: []*v1.AnyValue{
										{Value: &v1.AnyValue_StringValue{StringValue: "one"}},
										{Value: &v1.AnyValue_StringValue{StringValue: "two"}},
									},
								}}}},
							},
						},
						Spans: []*v1_trace.Span{{
							Name:   "span-a",
							SpanId: common.ID{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
							Attributes: []*v1.KeyValue{
								{Key: "span.attr", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "aaa"}}},
								{Key: "dedicated.span.1", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-span-attr-value-1"}}},
								{Key: "dedicated.span.2", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-span-attr-value-2"}}},
								{Key: "dedicated.span.3", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-span-attr-value-3"}}},
								{Key: "dedicated.span.4", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-span-attr-value-4"}}},
								{Key: "dedicated.span.5", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "dedicated-span-attr-value-5"}}},
								{Key: "span.string.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
									Values: []*v1.AnyValue{
										{Value: &v1.AnyValue_StringValue{StringValue: "one"}},
										{Value: &v1.AnyValue_StringValue{StringValue: "two"}},
									},
								}}}},
								{Key: "span.int.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
									Values: []*v1.AnyValue{
										{Value: &v1.AnyValue_IntValue{IntValue: 1}},
										{Value: &v1.AnyValue_IntValue{IntValue: 2}},
										{Value: &v1.AnyValue_IntValue{IntValue: 3}},
									},
								}}}},
								{Key: "span.double.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
									Values: []*v1.AnyValue{
										{Value: &v1.AnyValue_DoubleValue{DoubleValue: 1.1}},
										{Value: &v1.AnyValue_DoubleValue{DoubleValue: 2.2}},
									},
								}}}},
								{Key: "span.bool.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
									Values: []*v1.AnyValue{
										{Value: &v1.AnyValue_BoolValue{BoolValue: true}},
										{Value: &v1.AnyValue_BoolValue{BoolValue: false}},
										{Value: &v1.AnyValue_BoolValue{BoolValue: true}},
										{Value: &v1.AnyValue_BoolValue{BoolValue: false}},
									},
								}}}},
								{Key: "http.method", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "POST"}}},
								{Key: "http.url", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "https://example.com"}}},
								{Key: "http.status_code", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 201}}},
								{Key: "span.unsupported.array", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{
									Values: []*v1.AnyValue{
										{Value: &v1.AnyValue_BoolValue{BoolValue: true}},
										{Value: &v1.AnyValue_IntValue{IntValue: 1}},
										{Value: &v1.AnyValue_BoolValue{BoolValue: true}},
									},
								}}}},
								{Key: "span.unsupported.kvlist", Value: &v1.AnyValue{Value: &v1.AnyValue_KvlistValue{KvlistValue: &v1.KeyValueList{
									Values: []*v1.KeyValue{
										{Key: "key-a", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "val-a"}}},
										{Key: "key-b", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "val-b"}}},
									},
								}}}},
							},
						}},
					}},
				}},
			},
			// Verify that proto -> flat span round trip produces the expected flat span
			expected: nil, // Complex expected output validated via round-trip instead
		},
		{
			name: "nested set model bounds",
			id:   traceID,
			trace: tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{{
					Resource: &v1_resource.Resource{
						Attributes: []*v1.KeyValue{
							{Key: "service.name", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "service-a"}}},
						},
					},
					ScopeSpans: []*v1_trace.ScopeSpans{{
						Scope: &v1.InstrumentationScope{},
						Spans: []*v1_trace.Span{
							{
								Name:   "span-a",
								SpanId: common.ID{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
								Attributes: []*v1.KeyValue{
									{Key: "span.attr", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "aaa"}}},
								},
							},
							{
								Name:         "span-b",
								SpanId:       common.ID{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
								ParentSpanId: common.ID{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
								Attributes: []*v1.KeyValue{
									{Key: "span.attr", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "bbb"}}},
								},
							},
							{
								Name:         "span-c",
								SpanId:       common.ID{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03},
								ParentSpanId: common.ID{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
								Attributes: []*v1.KeyValue{
									{Key: "span.attr", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "ccc"}}},
								},
							},
						},
					}},
				}},
			},
			expected: nil, // Validated via round-trip
		},
	}

	for _, tt := range tsc {
		t.Run(tt.name, func(t *testing.T) {
			actual, _ := traceToParquet(&meta, tt.id, &tt.trace, nil)
			if tt.expected != nil {
				flatSpansEqual(t, tt.expected, actual)
			}
			// Round-trip: flat spans -> proto -> flat spans should be stable
			protoTrace := FlatSpansToTempopbTrace(&meta, flatSpanPtrs(actual))
			roundTripped, _ := traceToParquet(&meta, tt.id, protoTrace, nil)
			flatSpansEqual(t, actual, roundTripped)
		})
	}
}

func BenchmarkProtoToParquet(b *testing.B) {
	meta := backend.BlockMeta{
		DedicatedColumns: test.MakeDedicatedColumns(),
	}

	batchCount := 100
	spanCounts := []int{
		100, 1000,
		10000,
	}

	for _, spanCount := range spanCounts {
		b.Run("SpanCount:"+humanize.SI(float64(batchCount*spanCount), ""), func(b *testing.B) {
			id := test.ValidTraceID(nil)
			tr := test.AddDedicatedAttributes(test.MakeTraceWithSpanCount(batchCount, spanCount, id))

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = traceToParquet(&meta, id, tr, nil)
			}
		})
	}
}

func BenchmarkEventToParquet(b *testing.B) {
	s := &v1_trace.Span{
		StartTimeUnixNano: 100,
	}
	e := &v1_trace.Span_Event{
		TimeUnixNano: 1000,
		Name:         "blerg",
		Attributes: []*v1.KeyValue{
			{Key: "s", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "s2"}}},
			{Key: "i", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 123}}},
			{Key: "d", Value: &v1.AnyValue{Value: &v1.AnyValue_DoubleValue{DoubleValue: 123.456}}},
			{Key: "b", Value: &v1.AnyValue{Value: &v1.AnyValue_BoolValue{BoolValue: true}}},
			{Key: "kv", Value: &v1.AnyValue{Value: &v1.AnyValue_KvlistValue{KvlistValue: &v1.KeyValueList{Values: []*v1.KeyValue{
				{Key: "s2", Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: "s3"}}},
				{Key: "i2", Value: &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: 789}}},
			}}}}},
			{Key: "a", Value: &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: []*v1.AnyValue{
				{Value: &v1.AnyValue_StringValue{StringValue: "s4"}},
				{Value: &v1.AnyValue_IntValue{IntValue: 101112}},
			}}}}},
		},
	}

	ee := &Event{}
	for i := 0; i < b.N; i++ {
		eventToParquet(e, ee, s.StartTimeUnixNano, dedicatedColumnMapping{})
	}
}

func BenchmarkDeconstruct(b *testing.B) {
	meta := backend.BlockMeta{
		DedicatedColumns: test.MakeDedicatedColumns(),
	}

	batchCount := 100
	spanCounts := []int{
		100, 1000,
		10000,
	}

	poolSizes := []int{
		100_000,
		30_000_000,
	}

	for _, spanCount := range spanCounts {
		for _, poolSize := range poolSizes {
			ss := humanize.SI(float64(batchCount*spanCount), "")
			ps := humanize.SI(float64(poolSize), "")
			b.Run(fmt.Sprintf("SpanCount%v/Pool%v", ss, ps), func(b *testing.B) {
				id := test.ValidTraceID(nil)
				dbt := test.MakeTraceWithSpanCount(batchCount, spanCount, id)
				test.AddDedicatedAttributes(dbt)

				tr, _ := traceToParquet(&meta, id, dbt, nil)
				if len(tr) == 0 {
					b.Skip("no spans")
				}
				sch := parquet.SchemaOf(tr[0])

				b.ResetTimer()

				pool := newRowPool(poolSize)

				for i := 0; i < b.N; i++ {
					r2 := sch.Deconstruct(pool.Get(), &tr[0])
					pool.Put(r2)
				}
			})
		}
	}
}

func TestParquetRowSizeEstimate(t *testing.T) {
	// use this test to parse actual Parquet files and compare the two methods of estimating row size
	s := []string{}

	for _, s := range s {
		estimateRowSize(t, s)
	}
}

func estimateRowSize(t *testing.T, name string) {
	t.Helper()
	// Skipping: this test was designed for manual exploration of parquet files
	t.Skip("manual test - provide a file path")
}

func TestExtendReuseSlice(t *testing.T) {
	tcs := []struct {
		sz       int
		in       []int
		expected []int
	}{
		{
			sz:       0,
			in:       []int{1, 2, 3},
			expected: []int{},
		},
		{
			sz:       2,
			in:       []int{1, 2, 3},
			expected: []int{1, 2},
		},
		{
			sz:       5,
			in:       []int{1, 2, 3},
			expected: []int{1, 2, 3, 0, 0},
		},
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("%v", tc.sz), func(t *testing.T) {
			out := extendReuseSlice(tc.sz, tc.in)
			assert.Equal(t, tc.expected, out)
		})
	}
}

func BenchmarkExtendReuseSlice(b *testing.B) {
	in := []int{1, 2, 3}
	for i := 0; i < b.N; i++ {
		_ = extendReuseSlice(100, in)
	}
}

var benchmarkFlatSpans []FlatSpan

func BenchmarkTraceToParquet(b *testing.B) {
	var (
		traceID = test.ValidTraceID(nil)
		traces  = make([]*tempopb.Trace, 0, 1_000)
	)

	for range 1_000 {
		nb := 20 + rand.Intn(5)
		id := test.ValidTraceID(nil)
		traces = append(traces, test.AddDedicatedAttributes(test.MakeTrace(nb, test.ValidTraceID(id))))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		t := traces[i%len(traces)]

		spans, _ := traceToParquet(&backend.BlockMeta{}, traceID, t, nil)
		benchmarkFlatSpans = spans
	}
}

func tempopbTraceEqual(t *testing.T, expected, actual *tempopb.Trace) {
	t.Helper()
	sortAttributesTempopb(expected)
	sortAttributesTempopb(actual)

	if !proto.Equal(expected, actual) {
		t.Log(cmp.Diff(expected, actual))
		assert.Fail(t, "expected and actual are not equal")
	}
}

func sortAttributesTempopb(t *tempopb.Trace) {
	for _, rs := range t.ResourceSpans {
		sort.Slice(rs.Resource.Attributes, func(i, j int) bool {
			if rs.Resource.Attributes[i].Key == rs.Resource.Attributes[j].Key {
				return rs.Resource.Attributes[i].Value.String() < rs.Resource.Attributes[j].Value.String()
			}

			return rs.Resource.Attributes[i].Key < rs.Resource.Attributes[j].Key
		})
		for _, ss := range rs.ScopeSpans {
			sort.Slice(ss.Scope.Attributes, func(i, j int) bool {
				if rs.Resource.Attributes[i].Key == rs.Resource.Attributes[j].Key {
					return rs.Resource.Attributes[i].Value.String() < rs.Resource.Attributes[j].Value.String()
				}

				return ss.Scope.Attributes[i].Key < ss.Scope.Attributes[j].Key
			})
		}
	}
}

// flatSpansEqual asserts similar to assert.Equal but treats empty / nil slices and maps as equal
func flatSpansEqual(t *testing.T, expected, actual []FlatSpan, messages ...interface{}) {
	t.Helper()
	sortFlatSpanAttributes(expected)
	sortFlatSpanAttributes(actual)

	if !cmp.Equal(expected, actual, cmpopts.EquateEmpty()) {
		t.Log(cmp.Diff(expected, actual, cmpopts.EquateEmpty()))
		assert.Fail(t, "expected and actual are not equal", messages...)
	}
}

func sortFlatSpanAttributes(spans []FlatSpan) {
	for i := range spans {
		sort.Slice(spans[i].ResourceAttrs, func(a, b int) bool {
			return spans[i].ResourceAttrs[a].Key < spans[i].ResourceAttrs[b].Key
		})
		sort.Slice(spans[i].ScopeAttrs, func(a, b int) bool {
			return spans[i].ScopeAttrs[a].Key < spans[i].ScopeAttrs[b].Key
		})
		sort.Slice(spans[i].Attrs, func(a, b int) bool {
			return spans[i].Attrs[a].Key < spans[i].Attrs[b].Key
		})
	}
}

func TestTraceToParquetRootSpanWithChildOfLink(t *testing.T) {
	traceID := common.ID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}

	tsc := []struct {
		name             string
		trace            *tempopb.Trace
		expectedRootName string
	}{
		{
			name: "root span with a child-of link",
			trace: &tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{
					{
						Resource: &v1_resource.Resource{},
						ScopeSpans: []*v1_trace.ScopeSpans{
							{
								Scope: &v1.InstrumentationScope{},
								Spans: []*v1_trace.Span{
									{
										Name:   "not-root-span",
										SpanId: []byte{0x02},
										Links: []*v1_trace.Span_Link{
											{
												Attributes: []*v1.KeyValue{
													{
														Key: "opentracing.ref_type",
														Value: &v1.AnyValue{
															Value: &v1.AnyValue_StringValue{StringValue: "child_of"},
														},
													},
												},
											},
										},
									},
									{
										Name:   "root-span",
										SpanId: []byte{0x01},
									},
									{
										Name:         "child-span",
										SpanId:       []byte{0x03},
										ParentSpanId: []byte{0x01},
									},
								},
							},
						},
					},
				},
			},
			expectedRootName: "root-span",
		},
		{
			name: "root span without child-of link",
			trace: &tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{
					{
						Resource: &v1_resource.Resource{},
						ScopeSpans: []*v1_trace.ScopeSpans{
							{
								Scope: &v1.InstrumentationScope{},
								Spans: []*v1_trace.Span{
									{
										Name:   "root-span",
										SpanId: []byte{0x01},
									},
									{
										Name:         "child-span",
										SpanId:       []byte{0x02},
										ParentSpanId: []byte{0x01},
									},
								},
							},
						},
					},
				},
			},
			expectedRootName: "root-span",
		},
		{
			name: "span link different trace id",
			trace: &tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{
					{
						Resource: &v1_resource.Resource{},
						ScopeSpans: []*v1_trace.ScopeSpans{
							{
								Scope: &v1.InstrumentationScope{},
								Spans: []*v1_trace.Span{
									{
										Name:    "not-root-span",
										TraceId: traceID,
										SpanId:  []byte{0x02},
										Links: []*v1_trace.Span_Link{
											{
												TraceId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
												Attributes: []*v1.KeyValue{
													{
														Key: "opentracing.ref_type",
														Value: &v1.AnyValue{
															Value: &v1.AnyValue_StringValue{StringValue: "child_of"},
														},
													},
												},
											},
										},
									},
									{
										Name:    "root-span",
										SpanId:  []byte{0x01},
										TraceId: traceID,
									},
									{
										Name:         "child-span",
										SpanId:       []byte{0x03},
										ParentSpanId: []byte{0x01},
										TraceId:      traceID,
									},
								},
							},
						},
					},
				},
			},
			expectedRootName: "root-span",
		},
		{
			name: "no root span",
			trace: &tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{
					{
						Resource: &v1_resource.Resource{},
						ScopeSpans: []*v1_trace.ScopeSpans{
							{
								Scope: &v1.InstrumentationScope{},
								Spans: []*v1_trace.Span{
									{
										Name:         "child-span",
										SpanId:       []byte{0x02},
										ParentSpanId: []byte{0x01},
									},
								},
							},
						},
					},
				},
			},
			expectedRootName: "",
		},
		{
			name: "no spans",
			trace: &tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{
					{
						Resource:   &v1_resource.Resource{},
						ScopeSpans: []*v1_trace.ScopeSpans{},
					},
				},
			},
			expectedRootName: "",
		},
		{
			name: "span with parent but no root",
			trace: &tempopb.Trace{
				ResourceSpans: []*v1_trace.ResourceSpans{
					{
						Resource: &v1_resource.Resource{},
						ScopeSpans: []*v1_trace.ScopeSpans{
							{
								Scope: &v1.InstrumentationScope{},
								Spans: []*v1_trace.Span{
									{
										Name:         "child-span",
										SpanId:       []byte{0x02},
										ParentSpanId: []byte{0x01},
									},
								},
							},
						},
					},
				},
			},
			expectedRootName: "",
		},
	}

	for _, tt := range tsc {
		t.Run(tt.name, func(t *testing.T) {
			meta := backend.BlockMeta{DedicatedColumns: test.MakeDedicatedColumns()}
			parquetSpans, _ := traceToParquet(&meta, traceID, tt.trace, nil)
			if len(parquetSpans) > 0 {
				assert.Equal(t, tt.expectedRootName, parquetSpans[0].RootSpanName)
			} else {
				assert.Equal(t, tt.expectedRootName, "")
			}
		})
	}
}
