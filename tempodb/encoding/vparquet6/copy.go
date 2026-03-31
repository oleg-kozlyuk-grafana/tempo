package vparquet6

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/grafana/tempo/tempodb/backend"
)

func CopyBlock(ctx context.Context, fromMeta, toMeta *backend.BlockMeta, from backend.Reader, to backend.Writer) error {
	// Copy streams, efficient but can't cache.
	copyStream := func(name string) error {
		reader, size, err := from.StreamReader(ctx, name, (uuid.UUID)(fromMeta.BlockID), fromMeta.TenantID)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", name, err)
		}
		defer reader.Close()

		return to.StreamWriter(ctx, name, (uuid.UUID)(toMeta.BlockID), toMeta.TenantID, reader, size)
	}

	// Data
	err := copyStream(DataFileName)
	if err != nil {
		return err
	}

	// Meta
	err = to.WriteBlockMeta(ctx, toMeta)
	return err
}

func writeBlockMeta(ctx context.Context, w backend.Writer, meta *backend.BlockMeta) error {
	err := w.WriteBlockMeta(ctx, meta)
	if err != nil {
		return fmt.Errorf("unexpected error writing meta: %w", err)
	}

	return nil
}
