package vparquet6

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/encoding/common"
	"go.opentelemetry.io/otel"
)

const (
	DataFileName = "data.parquet"
)

var tracer = otel.Tracer("tempodb/encoding/vparquet6")

type backendBlock struct {
	meta *backend.BlockMeta
	r    backend.Reader

	openMtx sync.Mutex
}

var _ common.BackendBlock = (*backendBlock)(nil)

func newBackendBlock(meta *backend.BlockMeta, r backend.Reader) *backendBlock {
	return &backendBlock{
		meta: meta,
		r:    r,
	}
}

func (b *backendBlock) BlockMeta() *backend.BlockMeta {
	return b.meta
}

// Validate does a basic sanity check of the parquet file.
func (b *backendBlock) Validate(ctx context.Context) error {
	if b.meta == nil {
		return errors.New("block meta is nil")
	}

	// read last 8 bytes of the file to confirm its at least complete. the last 4 should be ascii "PAR1"
	// and the 4 bytes before that should be the length of the footer
	buff := make([]byte, 8)
	err := b.r.ReadRange(ctx, DataFileName, uuid.UUID(b.meta.BlockID), b.meta.TenantID, b.meta.Size_-8, buff, nil)
	if err != nil {
		return fmt.Errorf("failed to read parquet magic footer: %w", err)
	}

	if string(buff[4:]) != "PAR1" {
		return fmt.Errorf("invalid parquet magic footer: %x", buff[4:])
	}

	footerSize := int64(binary.LittleEndian.Uint32(buff[:4]))
	if footerSize != int64(b.meta.FooterSize) {
		return fmt.Errorf("unexpected parquet footer size: %d", footerSize)
	}

	return nil
}
