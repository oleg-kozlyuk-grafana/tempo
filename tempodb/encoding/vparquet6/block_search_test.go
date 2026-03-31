package vparquet6

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	tempo_io "github.com/grafana/tempo/pkg/io"
	"github.com/grafana/tempo/pkg/tempopb"
	common_v1 "github.com/grafana/tempo/pkg/tempopb/common/v1"
	v1 "github.com/grafana/tempo/pkg/tempopb/trace/v1"
	"github.com/grafana/tempo/pkg/util"
	"github.com/grafana/tempo/pkg/util/test"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/backend/local"
	"github.com/grafana/tempo/tempodb/encoding/common"
)

func TestBackendBlockSearch(t *testing.T) {
	t.Parallel()

	// Build a proto trace with all the attributes we want to search for
	wantTraceID := test.ValidTraceID(nil)
	wantProtoTrace := test.MakeTraceWithTags(wantTraceID, "myservice", 500)
	// Add attributes expected by the search tests
	for _, rs := range wantProtoTrace.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				s.Status = &v1.Status{Code: v1.Status_STATUS_CODE_ERROR}
				s.Attributes = append(s.Attributes,
					&common_v1.KeyValue{Key: "foo", Value: &common_v1.AnyValue{Value: &common_v1.AnyValue_StringValue{StringValue: "bar"}}},
					&common_v1.KeyValue{Key: LabelHTTPMethod, Value: &common_v1.AnyValue{Value: &common_v1.AnyValue_StringValue{StringValue: "get"}}},
					&common_v1.KeyValue{Key: LabelHTTPUrl, Value: &common_v1.AnyValue{Value: &common_v1.AnyValue_StringValue{StringValue: "url/hello/world"}}},
					&common_v1.KeyValue{Key: LabelHTTPStatusCode, Value: &common_v1.AnyValue{Value: &common_v1.AnyValue_IntValue{IntValue: 500}}},
				)
			}
		}
	}

	dc := test.MakeDedicatedColumns()
	meta := &backend.BlockMeta{DedicatedColumns: dc}
	wantFlatSpans, _ := traceToParquet(meta, wantTraceID, wantProtoTrace, nil)

	// make a bunch of traces and include our wantFlatSpans above
	total := 1000
	insertAt := rand.Intn(total)
	allTraces := make([][]FlatSpan, 0, total)
	for i := 0; i < total; i++ {
		if i == insertAt {
			allTraces = append(allTraces, wantFlatSpans)
			continue
		}

		id := test.ValidTraceID(nil)
		pbTrace := test.MakeTrace(10, id)
		pqSpans, _ := traceToParquet(meta, id, pbTrace, nil)
		allTraces = append(allTraces, pqSpans)
	}

	b := makeBackendBlockWithTraces(t, allTraces)
	ctx := context.TODO()

	// Helper function to make a tag search
	makeReq := func(k, v string) *tempopb.SearchRequest {
		return &tempopb.SearchRequest{
			Tags: map[string]string{
				k: v,
			},
		}
	}

	// Matches - use values from test.MakeTraceWithTags
	searchesThatMatch := []*tempopb.SearchRequest{
		{
			// Empty request
		},

		// Well-known resource attributes
		makeReq(LabelServiceName, "service"),

		// Well-known span attributes
		makeReq(LabelHTTPMethod, "get"),
		makeReq(LabelHTTPUrl, "url/hello/world"),
		makeReq(LabelStatusCode, StatusCodeError),

		// Span attributes
		makeReq("foo", "bar"),
	}

	traceIDText := util.TraceIDToHexString(wantTraceID)

	findInResults := func(id string, res []*tempopb.TraceSearchMetadata) *tempopb.TraceSearchMetadata {
		for _, r := range res {
			if r.TraceID == id {
				return r
			}
		}
		return nil
	}

	for _, req := range searchesThatMatch {
		res, err := b.Search(ctx, req, common.DefaultSearchOptions())
		require.NoError(t, err)

		meta := findInResults(traceIDText, res.Traces)
		require.NotNil(t, meta, "search request:", req)
	}

	// Excludes
	searchesThatDontMatch := []*tempopb.SearchRequest{
		// Well-known resource attributes
		makeReq(LabelServiceName, "foo"),

		// Former well-known span attributes
		makeReq(LabelHTTPMethod, "post"),
		makeReq(LabelHTTPUrl, "asdf"),
		makeReq(LabelStatusCode, StatusCodeOK),

		// Span attributes
		makeReq("foo", "baz"),

		// Multiple
		{
			Tags: map[string]string{
				"http.status_code": "500",
				"service.name":     "asdf",
			},
		},
	}
	for _, req := range searchesThatDontMatch {
		res, err := b.Search(ctx, req, common.DefaultSearchOptions())
		require.NoError(t, err)
		meta := findInResults(traceIDText, res.Traces)
		require.Nil(t, meta, req)
	}
}

func makeBackendBlockWithTraces(t *testing.T, trs [][]FlatSpan) *backendBlock {
	return makeBackendBlockWithTracesWithDedicatedColumns(t, trs, test.MakeDedicatedColumns())
}

func makeBackendBlockWithTracesWithDedicatedColumns(t *testing.T, trs [][]FlatSpan, dc backend.DedicatedColumns) *backendBlock {
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

	meta := backend.NewBlockMeta("fake", uuid.New(), VersionString)
	meta.TotalObjects = 1
	meta.DedicatedColumns = dc

	s, newMeta := newStreamingBlock(ctx, cfg, meta, r, w, tempo_io.NewBufferedWriter)

	for i, spans := range trs {
		var id []byte
		if len(spans) > 0 {
			id = spans[0].TraceID
		}
		err = s.Add(id, spans, 0, 0)
		require.NoError(t, err)
		if i%100 == 0 {
			_, err := s.Flush()
			require.NoError(t, err)
		}
	}

	_, err = s.Complete()
	require.NoError(t, err)

	b := newBackendBlock(newMeta, r)

	return b
}

func makeTraces() ([][]FlatSpan, map[string]string, map[string]string, map[string]string) {
	traces := [][]FlatSpan{}
	intrinsicVals := map[string]string{}
	resourceAttrVals := map[string]string{}
	spanAttrVals := map[string]string{}

	resourceAttrVals[LabelServiceName] = "servicename"
	intrinsicVals[LabelName] = "span"
	intrinsicVals[LabelRootServiceName] = "rootsvc"
	intrinsicVals[LabelStatusCode] = "2"
	intrinsicVals[LabelRootSpanName] = "rootspan"

	resourceAttrVals["dedicated.resource.1"] = "dedicated-resource-attr-value-1"
	resourceAttrVals["dedicated.resource.2"] = "dedicated-resource-attr-value-2"
	resourceAttrVals["dedicated.resource.3"] = "dedicated-resource-attr-value-3"
	resourceAttrVals["dedicated.resource.4"] = "dedicated-resource-attr-value-4"
	resourceAttrVals["dedicated.resource.5"] = "dedicated-resource-attr-value-5"
	spanAttrVals["dedicated.span.1"] = "dedicated-span-attr-value-1"
	spanAttrVals["dedicated.span.2"] = "dedicated-span-attr-value-2"
	spanAttrVals["dedicated.span.3"] = "dedicated-span-attr-value-3"
	spanAttrVals["dedicated.span.4"] = "dedicated-span-attr-value-4"
	spanAttrVals["dedicated.span.5"] = "dedicated-span-attr-value-5"

	for i := 0; i < 10; i++ {
		traceID := test.ValidTraceID(nil)
		traceIDText := util.TraceIDToHexString(traceID)
		var flatSpans []FlatSpan

		for j := 0; j < 3; j++ {
			resKey := test.RandomString()
			resVal := test.RandomString()
			resourceAttrVals[resKey] = resVal

			for k := 0; k < 10; k++ {
				spanKey := test.RandomString()
				spanVal := test.RandomString()
				spanAttrVals[spanKey] = spanVal

				fs := FlatSpan{
					TraceID:                traceID,
					TraceIDText:            traceIDText,
					TraceStartTimeUnixNano: uint64(1000 * time.Second),
					TraceEndTimeUnixNano:   uint64(2000 * time.Second),
					TraceDurationNano:      uint64(100 * time.Millisecond),
					RootServiceName:        "rootsvc",
					RootSpanName:           "rootspan",
					ResourceServiceName:    "servicename",
					ResourceAttrs: []Attribute{
						attr(resKey, resVal),
					},
					ResourceDedicatedString01: []string{"dedicated-resource-attr-value-1"},
					ResourceDedicatedString02: []string{"dedicated-resource-attr-value-2"},
					ResourceDedicatedString03: []string{"dedicated-resource-attr-value-3"},
					ResourceDedicatedString04: []string{"dedicated-resource-attr-value-4"},
					ResourceDedicatedString05: []string{"dedicated-resource-attr-value-5"},
					SpanID:                    test.ValidTraceID(nil)[:8],
					Name:                      "span",
					StatusCode:                2,
					StatusMessage:             "error",
					DurationNano:              uint64(100 * time.Second),
					Attrs: []Attribute{
						attr(spanKey, spanVal),
					},
					DedicatedString01: []string{"dedicated-span-attr-value-1"},
					DedicatedString02: []string{"dedicated-span-attr-value-2"},
					DedicatedString03: []string{"dedicated-span-attr-value-3"},
					DedicatedString04: []string{"dedicated-span-attr-value-4"},
					DedicatedString05: []string{"dedicated-span-attr-value-5"},
				}

				flatSpans = append(flatSpans, fs)
			}
		}

		traces = append(traces, flatSpans)
	}

	return traces, intrinsicVals, resourceAttrVals, spanAttrVals
}

func BenchmarkBackendBlockSearchTraces(b *testing.B) {
	testCases := []struct {
		name string
		tags map[string]string
	}{
		{"noMatch", map[string]string{"foo": "bar"}},
		{"partialMatch", map[string]string{"foo": "bar", "component": "gRPC"}},
		{"service.name", map[string]string{"service.name": "a"}},
	}

	ctx := context.TODO()
	block := blockForBenchmarks(b)

	opts := common.DefaultSearchOptions()
	opts.StartPage = 10
	opts.TotalPages = 10

	for _, tc := range testCases {

		req := &tempopb.SearchRequest{
			Tags:  tc.tags,
			Limit: 20,
		}

		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			bytesRead := 0
			for i := 0; i < b.N; i++ {
				resp, err := block.Search(ctx, req, opts)
				require.NoError(b, err)
				bytesRead += int(resp.Metrics.InspectedBytes)
			}
			b.SetBytes(int64(bytesRead) / int64(b.N))
			b.ReportMetric(float64(bytesRead)/float64(b.N), "bytes/op")
		})
	}
}
