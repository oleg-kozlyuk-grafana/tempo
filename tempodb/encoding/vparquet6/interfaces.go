package vparquet6

import (
	"context"

	"github.com/grafana/tempo/tempodb/encoding/common"
	"github.com/parquet-go/parquet-go"
)

type FlatSpanIterator interface {
	NextFlatSpan(context.Context) (common.ID, *FlatSpan, error)
	Close()
}

type RawIterator interface {
	Next(context.Context) (common.ID, parquet.Row, error)
	Close()
}
