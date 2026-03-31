package vparquet6

import (
	"testing"

	v1 "github.com/grafana/tempo/pkg/tempopb/trace/v1"
	"github.com/grafana/tempo/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignNestedSetModelBounds(t *testing.T) {
	tests := []struct {
		name              string
		spans             []FlatSpan
		expected          []FlatSpan
		expectedConnected bool
	}{
		{
			name: "single span",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 2, ParentID: -1, ChildCount: 0},
			},
			expectedConnected: true,
		},
		{
			name: "linear trace",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 6, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), NestedSetLeft: 2, NestedSetRight: 5, ParentID: 1, ChildCount: 1},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 3, NestedSetRight: 4, ParentID: 2, ChildCount: 0},
			},
			expectedConnected: true,
		},
		{
			name: "branched trace",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("eeeeeeee"), ParentSpanID: []byte("dddddddd")},
				{SpanID: []byte("ffffffff"), ParentSpanID: []byte("dddddddd")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 12, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), NestedSetLeft: 2, NestedSetRight: 11, ParentID: 1, ChildCount: 2},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 3, NestedSetRight: 4, ParentID: 2, ChildCount: 0},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 5, NestedSetRight: 10, ParentID: 2, ChildCount: 2},
				{SpanID: []byte("eeeeeeee"), ParentSpanID: []byte("dddddddd"), NestedSetLeft: 6, NestedSetRight: 7, ParentID: 5, ChildCount: 0},
				{SpanID: []byte("ffffffff"), ParentSpanID: []byte("dddddddd"), NestedSetLeft: 8, NestedSetRight: 9, ParentID: 5, ChildCount: 0},
			},
			expectedConnected: true,
		},
		{
			name: "multiple roots",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("eeeeeeee"), ParentSpanID: []byte("dddddddd")},
				{SpanID: []byte("ffffffff"), ParentSpanID: []byte("dddddddd")},

				{SpanID: []byte("gggggggg")},
				{SpanID: []byte("iiiiiiii"), ParentSpanID: []byte("hhhhhhhh")},
				{SpanID: []byte("hhhhhhhh"), ParentSpanID: []byte("gggggggg")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 12, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), NestedSetLeft: 2, NestedSetRight: 11, ParentID: 1, ChildCount: 2},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 3, NestedSetRight: 4, ParentID: 2, ChildCount: 0},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 5, NestedSetRight: 10, ParentID: 2, ChildCount: 2},
				{SpanID: []byte("eeeeeeee"), ParentSpanID: []byte("dddddddd"), NestedSetLeft: 6, NestedSetRight: 7, ParentID: 5, ChildCount: 0},
				{SpanID: []byte("ffffffff"), ParentSpanID: []byte("dddddddd"), NestedSetLeft: 8, NestedSetRight: 9, ParentID: 5, ChildCount: 0},

				{SpanID: []byte("gggggggg"), NestedSetLeft: 13, NestedSetRight: 18, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("hhhhhhhh"), ParentSpanID: []byte("gggggggg"), NestedSetLeft: 14, NestedSetRight: 17, ParentID: 13, ChildCount: 1},
				{SpanID: []byte("iiiiiiii"), ParentSpanID: []byte("hhhhhhhh"), NestedSetLeft: 15, NestedSetRight: 16, ParentID: 14, ChildCount: 0},
			},
			expectedConnected: true,
		},
		{
			name: "interrupted",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb")},

				{SpanID: []byte("eeeeeeee"), ParentSpanID: []byte("xxxxxxxx")}, // <- interrupted
				{SpanID: []byte("ffffffff"), ParentSpanID: []byte("eeeeeeee")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 8, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), NestedSetLeft: 2, NestedSetRight: 7, ParentID: 1, ChildCount: 2},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 3, NestedSetRight: 4, ParentID: 2, ChildCount: 0},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 5, NestedSetRight: 6, ParentID: 2, ChildCount: 0},

				{SpanID: []byte("eeeeeeee"), ParentSpanID: []byte("xxxxxxxx"), ChildCount: 1}, // <- interrupted
				{SpanID: []byte("ffffffff"), ParentSpanID: []byte("eeeeeeee"), ChildCount: 0},
			},
			expectedConnected: false,
		},
		{
			name: "partially assigned",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 4},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), NestedSetLeft: 2, NestedSetRight: 0, ParentID: 1},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 4, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), NestedSetLeft: 2, NestedSetRight: 3, ParentID: 1, ChildCount: 0},
			},
			expectedConnected: true,
		},
		{
			name: "non unique IDs",
			spans: []FlatSpan{
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), Kind: int(v1.Span_SPAN_KIND_CLIENT)},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb"), Kind: int(v1.Span_SPAN_KIND_SERVER)},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("aaaaaaaa")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 10, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb"), Kind: int(v1.Span_SPAN_KIND_SERVER), NestedSetLeft: 3, NestedSetRight: 8, ParentID: 2, ChildCount: 2},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), Kind: int(v1.Span_SPAN_KIND_CLIENT), NestedSetLeft: 2, NestedSetRight: 9, ParentID: 1, ChildCount: 1},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 4, NestedSetRight: 5, ParentID: 3, ChildCount: 0},
				{SpanID: []byte("dddddddd"), ParentSpanID: []byte("bbbbbbbb"), NestedSetLeft: 6, NestedSetRight: 7, ParentID: 3, ChildCount: 0},
			},
			expectedConnected: true,
		},
		{
			name: "non unique IDs 2x",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), Kind: int(v1.Span_SPAN_KIND_CLIENT)},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb"), Kind: int(v1.Span_SPAN_KIND_SERVER)},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("cccccccc"), Kind: int(v1.Span_SPAN_KIND_SERVER)},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb"), Kind: int(v1.Span_SPAN_KIND_CLIENT)},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa"), NestedSetLeft: 1, NestedSetRight: 10, ParentID: -1, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa"), Kind: int(v1.Span_SPAN_KIND_CLIENT), NestedSetLeft: 2, NestedSetRight: 9, ParentID: 1, ChildCount: 1},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb"), Kind: int(v1.Span_SPAN_KIND_CLIENT), NestedSetLeft: 4, NestedSetRight: 7, ParentID: 3, ChildCount: 1},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb"), Kind: int(v1.Span_SPAN_KIND_SERVER), NestedSetLeft: 3, NestedSetRight: 8, ParentID: 2, ChildCount: 1},
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("cccccccc"), Kind: int(v1.Span_SPAN_KIND_SERVER), NestedSetLeft: 5, NestedSetRight: 6, ParentID: 4, ChildCount: 0},
			},
			expectedConnected: true,
		},
		{
			name: "3x IDs - invalid trace",
			spans: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("aaaaaaaa")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("bbbbbbbb")},
			},
			expectedConnected: false,
		},
		{
			name: "no roots",
			spans: []FlatSpan{
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa")},
			},
			expected: []FlatSpan{
				{SpanID: []byte("cccccccc"), ParentSpanID: []byte("bbbbbbbb")},
				{SpanID: []byte("bbbbbbbb"), ParentSpanID: []byte("aaaaaaaa")},
			},
			expectedConnected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy so we don't mutate test data
			spans := make([]FlatSpan, len(tt.spans))
			copy(spans, tt.spans)
			connected := assignNestedSetModelBounds(spans)
			assertEqualNestedSetModelBounds(t, spans, tt.expected)
			assert.Equal(t, tt.expectedConnected, connected)
		})
	}
}

func assertEqualNestedSetModelBounds(t testing.TB, actual, expected []FlatSpan) {
	t.Helper()

	actualSpans := map[uint64]*FlatSpan{}
	for i := range actual {
		s := &actual[i]
		actualSpans[util.SpanIDAndKindToToken(s.SpanID, s.Kind)] = s
	}

	for _, exp := range expected {
		act, ok := actualSpans[util.SpanIDAndKindToToken(exp.SpanID, exp.Kind)]
		require.Truef(t, ok, "span '%v' expected but was missing", string(exp.SpanID))
		assert.Equalf(t, exp.NestedSetLeft, act.NestedSetLeft, "span '%v' NestedSetLeft is expected %d but was %d", string(exp.SpanID), exp.NestedSetLeft, act.NestedSetLeft)
		assert.Equalf(t, exp.NestedSetRight, act.NestedSetRight, "span '%v' NestedSetRight is expected %d but was %d", string(exp.SpanID), exp.NestedSetRight, act.NestedSetRight)
		assert.Equalf(t, exp.ParentID, act.ParentID, "span '%v' ParentID is expected %d but was %d", string(exp.SpanID), exp.ParentID, act.ParentID)
		assert.Equalf(t, exp.ChildCount, act.ChildCount, "span '%v' ChildCount is expected %d but was %d", string(exp.SpanID), exp.ChildCount, act.ChildCount)
		assert.Equalf(t, exp.ParentSpanID, act.ParentSpanID, "span '%v' ParentSpanID is expected %d but was %d", string(exp.SpanID), string(exp.ParentSpanID), string(act.ParentSpanID))
		assert.Equalf(t, exp.Kind, act.Kind, "span '%v' Kind is expected %d but was %d", string(exp.SpanID), exp.Kind, act.Kind)
	}

	assert.Equalf(t, len(expected), len(actual), "expected %d spans but found %d instead", len(expected), len(actual))
}
