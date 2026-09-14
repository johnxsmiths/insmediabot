package storage

import (
	"context"
	"io"
)

// Storage defines the interface for temporary file hosting providers.
type Storage interface {
	Name() string
	Upload(ctx context.Context, reader io.Reader, filename, contentType string) (string, error)
	Delete(ctx context.Context, fileIDOrURL string) error
}
