//go:build !cgo
// +build !cgo

package sqlite

import (
	"armyknife/internal/vector"
	"context"

	"golang.org/x/xerrors"
)

type Vec struct{}

func NewVec(path string, dimension int) (*Vec, error) {
	return nil, xerrors.New("sqlite-vec requires CGO to be enabled. Please build with CGO_ENABLED=1")
}

func (s *Vec) Index(ctx context.Context, id string, vector []float32, metadata map[string]string, source string) error {
	return xerrors.New("sqlite-vec requires CGO to be enabled")
}

func (s *Vec) Search(ctx context.Context, queryVector []float32, limit int) ([]*vector.SearchResult, error) {
	return nil, xerrors.New("sqlite-vec requires CGO to be enabled")
}

func (s *Vec) Exists(ctx context.Context, id string) (bool, error) {
	return false, xerrors.New("sqlite-vec requires CGO to be enabled")
}

func (s *Vec) Delete(ctx context.Context, id string) error {
	return xerrors.New("sqlite-vec requires CGO to be enabled")
}

func (s *Vec) Touch(ctx context.Context, ids []string) error {
	return xerrors.New("sqlite-vec requires CGO to be enabled")
}

func (s *Vec) DeleteOld(ctx context.Context, source string, olderThan int64) error {
	return xerrors.New("sqlite-vec requires CGO to be enabled")
}

func (s *Vec) Close() error {
	return nil
}
