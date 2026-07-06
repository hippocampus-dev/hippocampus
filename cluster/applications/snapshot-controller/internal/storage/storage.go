package storage

import (
	"context"
)

type Storage interface {
	// Put stores data with the given key and returns the storage URL
	Put(ctx context.Context, key string, data []byte) (string, error)
	// Get retrieves data from the given storage URL
	Get(ctx context.Context, url string) ([]byte, error)
}
