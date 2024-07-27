package lock

import (
	"context"
	"errors"
	"github.com/go-redsync/redsync/v4"
	"go.etcd.io/etcd/client/v3/concurrency"
)

var ErrLockAlreadyTaken = errors.New("lock already taken")

type Lock interface {
	Lock(context.Context) (*int64, error)
	Unlock(context.Context) error
	Value() string
}

type RedsyncWrapper struct {
	*redsync.Mutex
}

func (r *RedsyncWrapper) Lock(ctx context.Context) (*int64, error) {
	if err := r.Mutex.TryLockContext(ctx); err != nil {
		var errTaken *redsync.ErrTaken
		if errors.As(err, &errTaken) {
			return nil, ErrLockAlreadyTaken
		}
		return nil, err
	}
	return nil, nil
}

func (r *RedsyncWrapper) Unlock(ctx context.Context) error {
	_, err := r.Mutex.UnlockContext(ctx)
	if err != nil && !errors.Is(err, redsync.ErrLockAlreadyExpired) {
		return err
	}
	return nil
}

func (r *RedsyncWrapper) Value() string {
	return r.Mutex.Value()
}

type EtcdWrapper struct {
	*concurrency.Mutex
	Unlocker func(context.Context) error
}

func (e *EtcdWrapper) Lock(ctx context.Context) (*int64, error) {
	if err := e.Mutex.TryLock(ctx); err != nil {
		if errors.Is(err, concurrency.ErrLocked) {
			return nil, ErrLockAlreadyTaken
		}
		return nil, err
	}
	fencingToken := e.Mutex.Header().Revision
	return &fencingToken, nil
}

func (e *EtcdWrapper) Unlock(ctx context.Context) error {
	return e.Unlocker(ctx)
}

func (e *EtcdWrapper) Value() string {
	return e.Mutex.Key()
}
