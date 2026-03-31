package vparquet6

import (
	"bytes"
	"testing"

	pq "github.com/grafana/tempo/pkg/parquetquery"
	"github.com/grafana/tempo/pkg/util/test"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/require"
)

func TestCompressionCodecDataLoss(t *testing.T) {
	dc := test.MakeDedicatedColumns()
	meta := &backend.BlockMeta{DedicatedColumns: dc}
	id := test.ValidTraceID(nil)
	pbTrace := test.MakeTrace(30, id)
	allSpans, _ := traceToParquet(meta, id, pbTrace, nil)
	spans := allSpans[:30]

	codecs := []struct {
		name  string
		codec parquet.WriterOption
	}{
		{"no_compression", nil},
		{"snappy", parquet.Compression(&parquet.Snappy)},
		{"gzip", parquet.Compression(&parquet.Gzip)},
		{"lz4_raw", parquet.Compression(&parquet.Lz4Raw)},
		{"zstd", parquet.Compression(&parquet.Zstd)},
	}

	for _, tc := range codecs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "lz4_raw" {
				t.Skip("lz4_raw has a known bug in parquet-go where CompressBlock returns 0 bytes for incompressible data, silently dropping pages")
			}
			buf := new(bytes.Buffer)
			var writer *parquet.GenericWriter[*FlatSpan]
			if tc.codec != nil {
				writer = parquet.NewGenericWriter[*FlatSpan](buf, tc.codec)
			} else {
				writer = parquet.NewGenericWriter[*FlatSpan](buf)
			}

			ptrs := make([]*FlatSpan, len(spans))
			for i := range spans {
				ptrs[i] = &spans[i]
			}
			n, err := writer.Write(ptrs)
			require.NoError(t, err)
			require.Equal(t, len(spans), n)

			err = writer.Close()
			require.NoError(t, err)

			reader := bytes.NewReader(buf.Bytes())
			pf, err := parquet.OpenFile(reader, int64(buf.Len()))
			require.NoError(t, err)

			spanKeyIdx, _, _ := pq.GetColumnIndexByPath(pf, FieldSpanAttrKey)

			totalValues := int64(0)
			for _, rg := range pf.RowGroups() {
				cc := rg.ColumnChunks()[spanKeyIdx]
				pgs := cc.Pages()
				for {
					pg, err := pgs.ReadPage()
					if err != nil || pg == nil {
						break
					}
					totalValues += pg.NumValues()
					parquet.Release(pg)
				}
				pgs.Close()
			}
			t.Logf("totalAttrKeyValues=%d", totalValues)
			if totalValues == 0 {
				t.Error("DATA LOSS!")
			}
		})
	}
}
