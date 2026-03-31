package vparquet6

import (
	"bytes"
	"sort"

	"github.com/grafana/tempo/pkg/util"
)

// combineFlatSpans merges multiple sets of flat spans (same trace) into one, deduplicating by SpanID+Kind.
func combineFlatSpans(spanSets ...[]FlatSpan) []FlatSpan {
	if len(spanSets) == 1 {
		return spanSets[0]
	}

	c := NewCombiner()
	for i, spans := range spanSets {
		c.ConsumeWithFinal(spans, i == len(spanSets)-1)
	}
	res, _, _ := c.Result()
	return res
}

// Combiner combines multiple partial traces (as flat spans) into one, deduping spans based on
// ID and kind.
type Combiner struct {
	result   []FlatSpan
	spans    map[uint64]struct{}
	combined bool
}

func NewCombiner() *Combiner {
	return &Combiner{}
}

// Consume the given flat spans and destructively combines their contents.
func (c *Combiner) Consume(spans []FlatSpan) (spanCount int) {
	return c.ConsumeWithFinal(spans, false)
}

// ConsumeWithFinal consumes spans, but allows for performance savings when
// it is known that this is the last expected input.
func (c *Combiner) ConsumeWithFinal(spans []FlatSpan, final bool) (spanCount int) {
	if len(spans) == 0 {
		return
	}

	// First call
	if c.result == nil {
		c.result = spans
		c.spans = make(map[uint64]struct{}, len(spans))
		for _, s := range c.result {
			c.spans[util.SpanIDAndKindToToken(s.SpanID, s.Kind)] = struct{}{}
		}
		return
	}

	// Coalesce trace-level information from the incoming spans
	if len(spans) > 0 {
		first := &spans[0]
		if first.TraceEndTimeUnixNano > c.result[0].TraceEndTimeUnixNano {
			for i := range c.result {
				c.result[i].TraceEndTimeUnixNano = first.TraceEndTimeUnixNano
			}
		}
		if first.TraceStartTimeUnixNano < c.result[0].TraceStartTimeUnixNano || c.result[0].TraceStartTimeUnixNano == 0 {
			for i := range c.result {
				c.result[i].TraceStartTimeUnixNano = first.TraceStartTimeUnixNano
			}
		}
		if c.result[0].RootServiceName == "" && first.RootServiceName != "" {
			for i := range c.result {
				c.result[i].RootServiceName = first.RootServiceName
			}
		}
		if c.result[0].RootSpanName == "" && first.RootSpanName != "" {
			for i := range c.result {
				c.result[i].RootSpanName = first.RootSpanName
			}
		}
	}

	// Add non-duplicate spans
	for _, s := range spans {
		token := util.SpanIDAndKindToToken(s.SpanID, s.Kind)
		_, ok := c.spans[token]
		if !ok {
			c.result = append(c.result, s)
			if !final {
				c.spans[token] = struct{}{}
			}
		} else {
			spanCount++
		}
	}

	// Propagate resolved trace-level fields to all spans (including newly appended ones)
	if len(c.result) > 0 {
		ref := c.result[0]
		for i := range c.result {
			c.result[i].TraceStartTimeUnixNano = ref.TraceStartTimeUnixNano
			c.result[i].TraceEndTimeUnixNano = ref.TraceEndTimeUnixNano
			c.result[i].RootServiceName = ref.RootServiceName
			c.result[i].RootSpanName = ref.RootSpanName
		}
	}

	c.combined = true
	return
}

// Result returns the final flat spans, span count, and whether the trace is connected.
func (c *Combiner) Result() ([]FlatSpan, int, bool) {
	spanCount := -1

	connected := true
	if c.result != nil && c.combined {
		SortFlatSpans(c.result)
		connected = assignNestedSetModelBounds(c.result)
		spanCount = len(c.result)

		// Update trace duration
		if len(c.result) > 0 {
			dur := c.result[0].TraceEndTimeUnixNano - c.result[0].TraceStartTimeUnixNano
			for i := range c.result {
				c.result[i].TraceDurationNano = dur
			}
		}
	}

	return c.result, spanCount, connected
}

// SortFlatSpans sorts flat spans by start time, then SpanID.
func SortFlatSpans(spans []FlatSpan) {
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].StartTimeUnixNano == spans[j].StartTimeUnixNano {
			return bytes.Compare(spans[i].SpanID, spans[j].SpanID) == -1
		}
		return spans[i].StartTimeUnixNano < spans[j].StartTimeUnixNano
	})
}
