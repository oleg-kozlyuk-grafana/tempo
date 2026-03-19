package livestore

import (
	"context"

	"github.com/grafana/tempo/pkg/tempopb"
	"github.com/grafana/tempo/tempodb/encoding/common"
)

// traceBufferIter iterates over a slice of bufferedTraces, exposing a common.Iterator
// interface. Traces must be pre-sorted by ID before construction.
type traceBufferIter struct {
	traces []*bufferedTrace
	pos    int
}

func newTraceBufferIter(traces []*bufferedTrace) *traceBufferIter {
	return &traceBufferIter{traces: traces}
}

func (i *traceBufferIter) Next(_ context.Context) (common.ID, *tempopb.Trace, error) {
	if i.pos >= len(i.traces) {
		return nil, nil, nil
	}
	t := i.traces[i.pos]
	i.pos++
	return t.ID, t.Trace, nil
}

func (i *traceBufferIter) Close() {}

var _ common.Iterator = (*traceBufferIter)(nil)
