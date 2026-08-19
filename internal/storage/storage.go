package storage

import (
	"context"
	"io"
)

// ObjectStorage stores binary objects (e.g. channel thumbnails) behind a public URL.
type ObjectStorage interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) (publicURL string, err error)
	Delete(ctx context.Context, key string) error
}
