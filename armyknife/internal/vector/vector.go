package vector

import (
	"context"
)

type Store interface {
	Index(ctx context.Context, id string, vector []float32, metadata map[string]string, source string) error
	Search(ctx context.Context, vector []float32, limit int) ([]*SearchResult, error)
	Exists(ctx context.Context, id string) (bool, error)
	Delete(ctx context.Context, id string) error
	Touch(ctx context.Context, ids []string) error
	DeleteOld(ctx context.Context, source string, olderThan int64) error
	Close() error
}

type SearchResult struct {
	ID         string
	Metadata   map[string]string
	Distance   float64
	Similarity float64
}
