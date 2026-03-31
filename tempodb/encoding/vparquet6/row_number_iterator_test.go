package vparquet6

import (
	"bytes"
	"context"
	"testing"

	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/require"

	pq "github.com/grafana/tempo/pkg/parquetquery"
)

func TestRowNumberIterator_NestedEvents(t *testing.T) {
	// In the flat schema, each row is a FlatSpan with optional nested Events.
	// Test that parquet iterators produce correct row numbers for nested event columns.
	spans := []FlatSpan{
		{SpanID: []byte{1}, StatusCode: 0, Events: []Event{{Name: "e1"}, {Name: "e2"}}},
		{SpanID: []byte{2}, StatusCode: 0},
		{SpanID: []byte{3}, StatusCode: 0, Events: []Event{{Name: "e3"}, {Name: "e4"}, {Name: "e5"}}},
		{SpanID: []byte{4}, StatusCode: 0, Events: []Event{{Name: "e6"}}},
	}

	// Expected row numbers at the event level (definition level 1 for Events within FlatSpan):
	// Row 0: 2 events -> (0,0), (0,1)
	// Row 1: 0 events -> (1,-1) empty
	// Row 2: 3 events -> (2,0), (2,1), (2,2)
	// Row 3: 1 event  -> (3,0)
	eventRowNumbers := []pq.RowNumber{
		{0, 0, -1, -1, -1, -1, -1, -1},
		{0, 1, -1, -1, -1, -1, -1, -1},
		{1, -1, -1, -1, -1, -1, -1, -1},
		{2, 0, -1, -1, -1, -1, -1, -1},
		{2, 1, -1, -1, -1, -1, -1, -1},
		{2, 2, -1, -1, -1, -1, -1, -1},
		{3, 0, -1, -1, -1, -1, -1, -1},
	}

	pf := makeFlatTestFile(t, spans)
	makeIterator := makeIterFunc(context.Background(), pf.RowGroups(), pf)

	// Iterate over the event name column as a real iterator to verify against
	expectIter := makeIterator("Events.Name", nil, "eventName")

	for _, expRowNumber := range eventRowNumbers {
		exp, err := expectIter.Next()
		require.NoError(t, err)
		if exp == nil {
			break
		}
		require.Equal(t, expRowNumber, exp.RowNumber)
	}
}

func TestRowNumberIterator_BasicFlatSpan(t *testing.T) {
	// Basic test: flat spans with StatusCode column (scalar per row, no nesting).
	// Each row at level 0 should produce one result.
	spans := []FlatSpan{
		{SpanID: []byte{1}, StatusCode: 1},
		{SpanID: []byte{2}, StatusCode: 2},
		{SpanID: []byte{3}, StatusCode: 3},
	}

	pf := makeFlatTestFile(t, spans)
	makeIterator := makeIterFunc(context.Background(), pf.RowGroups(), pf)

	iter := makeIterator(columnPathSpanStatusCode, nil, "statusCode")

	expectedRows := []pq.RowNumber{
		{0, -1, -1, -1, -1, -1, -1, -1},
		{1, -1, -1, -1, -1, -1, -1, -1},
		{2, -1, -1, -1, -1, -1, -1, -1},
	}

	for _, expRowNumber := range expectedRows {
		res, err := iter.Next()
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, expRowNumber, res.RowNumber)
	}

	res, err := iter.Next()
	require.NoError(t, err)
	require.Nil(t, res)
}

func makeFlatTestFile(t testing.TB, spans []FlatSpan) *parquet.File {
	var buf bytes.Buffer

	w := parquet.NewGenericWriter[FlatSpan](&buf)
	n, err := w.Write(spans)
	require.NoError(t, err)
	require.Equal(t, len(spans), n)

	err = w.Close()
	require.NoError(t, err)

	data := buf.Bytes()
	pf, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	return pf
}
