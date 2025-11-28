//go:build !cgo
// +build !cgo

package sqlite

import (
	"armyknife/internal/vector"
	"context"

	"golang.org/x/xerrors"
)

// Vec is a stub implementation when CGO is disabled
type Vec struct{}

// NewVec returns an error when CGO is disabled
func NewVec(path string, dimension int) (*Vec, error) {
	return nil, xerrors.New("sqlite-vec requires CGO to be enabled. Please build with CGO_ENABLED=1")
}

// Index returns an error when CGO is disabled
func (s *Vec) Index(ctx context.Context, id string, vector []float32, metadata map[string]string, source string) error {
	return xerrors.New("sqlite-vec requires CGO to be enabled")
}

// Search returns an error when CGO is disabled
func (s *Vec) Search(ctx context.Context, queryVector []float32, limit int) ([]*vector.SearchResult, error) {
	return nil, xerrors.New("sqlite-vec requires CGO to be enabled")
}

// Exists returns an error when CGO is disabled
func (s *Vec) Exists(ctx context.Context, id string) (bool, error) {
	return false, xerrors.New("sqlite-vec requires CGO to be enabled")
}

// Delete returns an error when CGO is disabled
func (s *Vec) Delete(ctx context.Context, id string) error {
	return xerrors.New("sqlite-vec requires CGO to be enabled")
}

// DeleteOldEntries returns an error when CGO is disabled
func (s *Vec) DeleteOldEntries(ctx context.Context, source string, beforeTimestamp int64) error {
	return xerrors.New("sqlite-vec requires CGO to be enabled")
}

// Close returns nil when CGO is disabled
func (s *Vec) Close() error {
	return nil
}