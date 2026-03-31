package vparquet6

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/grafana/tempo/pkg/parquetquery"
	"github.com/grafana/tempo/pkg/traceql"
	"github.com/grafana/tempo/pkg/util"
	"github.com/grafana/tempo/tempodb/encoding/common"
	"github.com/stretchr/testify/require"
)

func TestBackendBlockSearchFetchMetaData(t *testing.T) {
	wantTr := fullyPopulatedTestTraceWithOption(nil, true)
	b := makeBackendBlockWithTraces(t, [][]FlatSpan{wantTr})
	ctx := context.Background()

	// Helper functions to make requests

	makeSpansets := func(sets ...*traceql.Spanset) []*traceql.Spanset {
		return sets
	}

	makeSpanset := func(traceID []byte, rootSpanName, rootServiceName string, startTimeUnixNano, durationNanos uint64, spans ...traceql.Span) *traceql.Spanset {
		return &traceql.Spanset{
			TraceID:            traceID,
			RootSpanName:       rootSpanName,
			RootServiceName:    rootServiceName,
			StartTimeUnixNanos: startTimeUnixNano,
			DurationNanos:      durationNanos,
			ServiceStats: map[string]traceql.ServiceStats{
				"myservice": {
					SpanCount:  1,
					ErrorCount: 0,
				},
				"service2": {
					SpanCount:  1,
					ErrorCount: 0,
				},
			},
			Spans: spans,
		}
	}

	testCases := []struct {
		name            string
		req             traceql.FetchSpansRequest
		expectedResults []*traceql.Spanset
	}{
		{
			"Empty request returns 1 spanset with all spans",
			makeReq(),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
					&span{
						id:                 wantTr[1].SpanID,
						startTimeUnixNanos: wantTr[1].StartTimeUnixNano,
						durationNanos:      wantTr[1].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Span attributes lookup",
			// Only matches 1 condition. Returns span but only attributes that matched
			makeReq(
				parse(t, `{span.foo = "bar"}`), // matches resource but not span
				parse(t, `{span.bar = 123}`),   // matches
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeSpan, false, "foo"), traceql.NewStaticNil()},
							{traceql.NewScopedAttribute(traceql.AttributeScopeSpan, false, "bar"), traceql.NewStaticInt(123)},
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Resource attributes lookup",
			makeReq(
				parse(t, `{resource.foo = "abc"}`), // matches resource but not span
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						resourceAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeResource, false, "foo"), traceql.NewStaticString("abc")},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Multiple attributes, only 1 matches and is returned",
			makeReq(
				parse(t, `{.foo = "xyz"}`),                   // doesn't match anything
				parse(t, `{.`+LabelHTTPStatusCode+` = 500}`), // matches span
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeSpan, false, "foo"), traceql.NewStaticNil()},
							{newSpanAttr(LabelHTTPStatusCode), traceql.NewStaticInt(500)}, // This is the only attribute that matched anything
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						resourceAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeResource, false, "foo"), traceql.NewStaticNil()},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Project attributes of all types",
			makeReq(
				parse(t, `{.foo }`),                    // String
				parse(t, `{.`+LabelHTTPStatusCode+`}`), // Int
				parse(t, `{.float }`),                  // Float
				parse(t, `{.bool }`),                   // bool
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeSpan, false, "foo"), traceql.NewStaticString("def")},
							{newSpanAttr("float"), traceql.NewStaticFloat(456.78)},
							{newSpanAttr("bool"), traceql.NewStaticBool(false)},
							{newSpanAttr(LabelHTTPStatusCode), traceql.NewStaticInt(500)}, // This is the only attribute that matched anything
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						resourceAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeResource, false, "foo"), traceql.NewStaticString("abc")},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
					&span{
						id:                 wantTr[1].SpanID,
						startTimeUnixNanos: wantTr[1].StartTimeUnixNano,
						durationNanos:      wantTr[1].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeSpan, false, "foo"), traceql.NewStaticString("ghi")},
							{newSpanAttr("float"), traceql.NewStaticFloat(456.789)},
							{newSpanAttr("bool"), traceql.NewStaticBool(true)},
							{newSpanAttr(LabelHTTPStatusCode), traceql.NewStaticInt(501)}, // This is the only attribute that matched anything
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
						},
						resourceAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeResource, false, "foo"), traceql.NewStaticString("abc2")},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Doesn't match anything",
			makeReq(parse(t, `{.xyz = "xyz"}`)),
			nil,
		},
		{
			"Intrinsics. 2nd span only",
			makeReq(
				parse(t, `{ name = "world" }`),
				parse(t, `{ status = unset }`),
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[1].SpanID,
						startTimeUnixNanos: wantTr[1].StartTimeUnixNano,
						durationNanos:      wantTr[1].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicName), traceql.NewStaticString("world")},
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Intrinsic duration with no filtering",
			makeReq(traceql.Condition{Attribute: traceql.NewIntrinsic(traceql.IntrinsicDuration)}),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							// duration exists twice on the span attrs b/c it's requested twice. once in the normal fetch conditions and once in the second
							// pass conditions. the actual engine code removes meta conditions based on the actual conditions so this won't normally happen
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
					&span{
						id:                 wantTr[1].SpanID,
						startTimeUnixNanos: wantTr[1].StartTimeUnixNano,
						durationNanos:      wantTr[1].DurationNano,
						spanAttrs: []attrVal{
							// duration exists twice on the span attrs b/c it's requested twice. once in the normal fetch conditions and once in the second
							// pass conditions. the actual engine code removes meta conditions based on the actual conditions so this won't normally happen
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Intrinsic span id with no filtering",
			makeReq(traceql.Condition{Attribute: traceql.NewIntrinsic(traceql.IntrinsicSpanID)}),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
					&span{
						id:                 wantTr[1].SpanID,
						startTimeUnixNanos: wantTr[1].StartTimeUnixNano,
						durationNanos:      wantTr[1].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Intrinsic span id for first span only",
			makeReq(parse(t, `{ span:id = "`+util.SpanIDToHexString(wantTr[0].SpanID)+`" }`)),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Intrinsic span parent id for first span only",
			makeReq(parse(t, `{ span:parentID = "`+util.SpanIDToHexString(wantTr[0].ParentSpanID)+`" }`)),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
							{traceql.NewIntrinsic(traceql.IntrinsicParentID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].ParentSpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Intrinsic trace id with no filtering",
			makeReq(traceql.Condition{Attribute: traceql.NewIntrinsic(traceql.IntrinsicTraceID)}),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
					&span{
						id:                 wantTr[1].SpanID,
						startTimeUnixNanos: wantTr[1].StartTimeUnixNano,
						durationNanos:      wantTr[1].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Intrinsic trace id for this trace",
			makeReq(parse(t, `{ trace:id = "`+util.TraceIDToHexString(wantTr[0].TraceID)+`" }`)),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
					&span{
						id:                 wantTr[1].SpanID,
						startTimeUnixNanos: wantTr[1].StartTimeUnixNano,
						durationNanos:      wantTr[1].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(200 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[1].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
					},
				),
			),
		},
		{
			"Intrinsic event name lookup",
			makeReq(
				parse(t, `{event:name = "e1"}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						eventAttrs: []attrVal{
							{traceql.IntrinsicEventNameAttribute, traceql.NewStaticString("e1")},
							{traceql.IntrinsicEventNameAttribute, traceql.NewStaticString("e1")}, // two events with the same name in the same span
						},
					},
				),
			),
		},
		{
			"Intrinsic event time since start lookup",
			makeReq(
				parse(t, `{event:timeSinceStart > 2ms}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						eventAttrs: []attrVal{
							{traceql.IntrinsicEventTimeSinceStartAttribute, traceql.NewStaticDuration(3 * time.Millisecond)},
							{traceql.IntrinsicEventTimeSinceStartAttribute, traceql.NewStaticDuration(3 * time.Millisecond)}, // two events with same time since start in the same span
						},
					},
				),
			),
		},
		{
			"Event attribute lookup",
			makeReq(
				parse(t, `{event.message =~ "excepti.*"}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						eventAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeEvent, false, "message"), traceql.NewStaticString("exception")},
							{traceql.NewScopedAttribute(traceql.AttributeScopeEvent, false, "message"), traceql.NewStaticString("exception")}, // two events with the same message attr in the same span
						},
					},
				),
			),
		},
		{
			"Intrinsic link trace ID lookup",
			makeReq(
				parse(t, `{link:traceID = "1234567890abcdef1234567890abcdef"}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						linkAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicLinkTraceID), traceql.NewStaticString("1234567890abcdef1234567890abcdef")},
						},
					},
				),
			),
		},
		{
			"Intrinsic link span ID lookup",
			makeReq(
				parse(t, `{link:spanID = "1234567890abcdef"}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						linkAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicLinkSpanID), traceql.NewStaticString("1234567890abcdef")},
						},
					},
				),
			),
		},
		{
			"Link attribute lookup",
			makeReq(
				parse(t, `{link.opentracing.ref_type =~ "child.*"}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						linkAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeLink, false, "opentracing.ref_type"), traceql.NewStaticString("child-of")},
						},
					},
				),
			),
		},
		{
			"Intrinsic instrumentation name lookup",
			makeReq(
				parse(t, `{instrumentation:name = "scope-1"}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						instrumentationAttrs: []attrVal{
							{traceql.IntrinsicInstrumentationNameAttribute, traceql.NewStaticString("scope-1")},
						},
					},
				),
			),
		},
		{
			"Intrinsic instrumentation version lookup",
			makeReq(
				parse(t, `{instrumentation:version = "version-1"}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						instrumentationAttrs: []attrVal{
							{traceql.IntrinsicInstrumentationVersionAttribute, traceql.NewStaticString("version-1")},
						},
					},
				),
			),
		},
		{
			"Instrumentation attribute lookup",
			makeReq(
				parse(t, `{instrumentation.scope-attr-int = 101}`), //
			),
			makeSpansets(
				makeSpanset(
					wantTr[0].TraceID,
					wantTr[0].RootSpanName,
					wantTr[0].RootServiceName,
					wantTr[0].TraceStartTimeUnixNano,
					wantTr[0].TraceDurationNano,
					&span{
						id:                 wantTr[0].SpanID,
						startTimeUnixNanos: wantTr[0].StartTimeUnixNano,
						durationNanos:      wantTr[0].DurationNano,
						spanAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicDuration), traceql.NewStaticDuration(100 * time.Second)},
							{traceql.NewIntrinsic(traceql.IntrinsicSpanID), traceql.NewStaticString(util.SpanIDToHexString(wantTr[0].SpanID))},
						},
						traceAttrs: []attrVal{
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootService), traceql.NewStaticString("RootService")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceRootSpan), traceql.NewStaticString("RootSpan")},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceDuration), traceql.NewStaticDuration(100 * time.Millisecond)},
							{traceql.NewIntrinsic(traceql.IntrinsicTraceID), traceql.NewStaticString(util.TraceIDToHexString(wantTr[0].TraceID))},
						},
						instrumentationAttrs: []attrVal{
							{traceql.NewScopedAttribute(traceql.AttributeScopeInstrumentation, false, "scope-attr-int"), traceql.NewStaticInt(101)},
						},
					},
				),
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.req
			resp, err := b.Fetch(ctx, req, common.DefaultSearchOptions())
			require.NoError(t, err, "search request:", req)

			// Turn iterator into slice
			var ss []*traceql.Spanset
			for {
				spanSet, err := resp.Results.Next(ctx)
				require.NoError(t, err)
				if spanSet == nil {
					break
				}
				ss = append(ss, spanSet)
			}

			// Clean up span internal details for consistent comparison.
			// (1) Wipe out internal fields like rownum.
			// (2) Wipe out empty leftover buffers from pooling.
			for _, s := range ss {
				for _, sp := range s.Spans {
					spn := sp.(*span)
					spn.cbSpanset = nil
					spn.cbSpansetFinal = false
					spn.rowNum = parquetquery.RowNumber{}

					if len(spn.traceAttrs) == 0 {
						spn.traceAttrs = nil
					}
					if len(spn.resourceAttrs) == 0 {
						spn.resourceAttrs = nil
					}
					if len(spn.spanAttrs) == 0 {
						spn.spanAttrs = nil
					}
					if len(spn.linkAttrs) == 0 {
						spn.linkAttrs = nil
					}
					if len(spn.eventAttrs) == 0 {
						spn.eventAttrs = nil
					}
					if len(spn.instrumentationAttrs) == 0 {
						spn.instrumentationAttrs = nil
					}

					// sort actual attrs to get consistent comparisons
					sortSpanAttrs(spn)
				}
				s.ReleaseFn = nil
			}

			// sort expected attrs to get consistent comparisons
			for _, s := range tc.expectedResults {
				for _, sp := range s.Spans {
					sortSpanAttrs(sp.(*span))
				}
			}
			require.Equal(t, tc.expectedResults, ss, "search request:", req)
		})
	}
}

func sortSpanAttrs(s *span) {
	// create sort func
	sortFn := func(a, b attrVal) bool {
		return a.a.String() < b.a.String()
	}
	// sort
	sort.Slice(s.spanAttrs, func(i, j int) bool {
		return sortFn(s.spanAttrs[i], s.spanAttrs[j])
	})
	sort.Slice(s.resourceAttrs, func(i, j int) bool {
		return sortFn(s.resourceAttrs[i], s.resourceAttrs[j])
	})
	sort.Slice(s.traceAttrs, func(i, j int) bool {
		return sortFn(s.traceAttrs[i], s.traceAttrs[j])
	})
}
