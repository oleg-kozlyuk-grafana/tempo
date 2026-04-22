package tempopb

import (
	"testing"

	v1common "github.com/grafana/tempo/pkg/tempopb/common/v1"
	v1resource "github.com/grafana/tempo/pkg/tempopb/resource/v1"
	v1 "github.com/grafana/tempo/pkg/tempopb/trace/v1"
)

// makeTestTrace creates a realistic trace with the given number of spans.
func makeTestTrace(numSpans int) *Trace {
	attrs := []v1common.KeyValue{
		{Key: "http.method", Value: v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: "GET"}}},
		{Key: "http.url", Value: v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: "https://example.com/api/v1/users"}}},
		{Key: "http.status_code", Value: v1common.AnyValue{Value: &v1common.AnyValue_IntValue{IntValue: 200}}},
		{Key: "component", Value: v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: "net/http"}}},
	}

	spans := make([]v1.Span, numSpans)
	for i := range spans {
		spans[i] = v1.Span{
			TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
			ParentSpanId:      []byte{8, 7, 6, 5, 4, 3, 2, 1},
			Name:              "HTTP GET /api/v1/users",
			Kind:              v1.Span_SPAN_KIND_SERVER,
			StartTimeUnixNano: 1_700_000_000_000_000_000,
			EndTimeUnixNano:   1_700_000_000_050_000_000,
			Attributes:        attrs,
			Status: v1.Status{
				Code: v1.Status_STATUS_CODE_OK,
			},
			Events: []v1.Span_Event{
				{
					TimeUnixNano: 1_700_000_000_010_000_000,
					Name:         "request.start",
					Attributes: []v1common.KeyValue{
						{Key: "event.attr", Value: v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: "value"}}},
					},
				},
			},
		}
	}

	return &Trace{
		ResourceSpans: []*v1.ResourceSpans{
			{
				Resource: v1resource.Resource{
					Attributes: []v1common.KeyValue{
						{Key: "service.name", Value: v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: "my-service"}}},
						{Key: "host.name", Value: v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: "host-001"}}},
					},
				},
				ScopeSpans: []v1.ScopeSpans{
					{
						Scope: v1common.InstrumentationScope{
							Name:    "go.opentelemetry.io/contrib",
							Version: "1.0.0",
						},
						Spans: spans,
					},
				},
			},
		},
	}
}

func BenchmarkTraceMarshal_10Spans(b *testing.B) {
	trace := makeTestTrace(10)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := trace.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTraceMarshal_100Spans(b *testing.B) {
	trace := makeTestTrace(100)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := trace.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTraceUnmarshal_10Spans(b *testing.B) {
	trace := makeTestTrace(10)
	data, _ := trace.Marshal()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		t := &Trace{}
		if err := t.Unmarshal(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTraceUnmarshal_100Spans(b *testing.B) {
	trace := makeTestTrace(100)
	data, _ := trace.Marshal()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		t := &Trace{}
		if err := t.Unmarshal(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResourceSpansMarshal_100Spans(b *testing.B) {
	trace := makeTestTrace(100)
	rs := trace.ResourceSpans[0]
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := rs.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResourceSpansUnmarshal_100Spans(b *testing.B) {
	trace := makeTestTrace(100)
	data, _ := trace.ResourceSpans[0].Marshal()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		rs := &v1.ResourceSpans{}
		if err := rs.Unmarshal(data); err != nil {
			b.Fatal(err)
		}
	}
}
