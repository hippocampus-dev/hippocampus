package sidecar

import (
	"context"
	"errors"
	"exactly-one-pod-hook/internal/lock"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/go-redsync/redsync/v4"
	redsyncredis "github.com/go-redsync/redsync/v4/redis"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"golang.org/x/xerrors"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		var errs validator.ValidationErrors
		errors.As(err, &errs)
		var messages []string
		for _, e := range errs {
			if e.ActualTag() == "oneof" {
				messages = append(messages, fmt.Sprintf("%s must be one of these [%s]", e.Field(), e.Param()))
			}
		}
		if len(messages) > 0 {
			err = xerrors.Errorf("%s: %w", strings.Join(messages, ", "), err)
		}
		return xerrors.Errorf("validation error: %w", err)
	}

	var unlockFactory func(key string, value string) (lock.Lock, error)

	switch a.LockMode {
	case "redlock":
		var redisPools []redsyncredis.Pool
		for _, addr := range a.RedisAddresses {
			redisClient := redis.NewClient(&redis.Options{
				Addr: addr,
			})
			redisPools = append(redisPools, redsyncgoredis.NewPool(redisClient))
		}
		redlock := redsync.New(redisPools...)

		unlockFactory = func(key string, value string) (lock.Lock, error) {
			return &lock.RedsyncWrapper{Mutex: redlock.NewMutex(key, redsync.WithValue(value))}, nil
		}
	case "etcd":
		etcdClient, err := clientv3.New(clientv3.Config{
			Endpoints: a.EtcdEndpoints,
		})
		if err != nil {
			return xerrors.Errorf("unable to create etcd client: %w", err)
		}
		defer etcdClient.Close()

		unlockFactory = func(key string, value string) (lock.Lock, error) {
			session, err := concurrency.NewSession(etcdClient)
			if err != nil {
				return nil, xerrors.Errorf("unable to create etcd session: %w", err)
			}
			session.Orphan()

			unlocker := func(ctx context.Context) error {
				_, err := session.Client().Delete(ctx, value)
				return err
			}

			return &lock.EtcdWrapper{Mutex: concurrency.NewMutex(session, key), Unlocker: unlocker}, nil
		}
	default:
		return xerrors.Errorf("invalid lock mode: %s", a.LockMode)
	}

	mutex, err := unlockFactory(a.UnlockKey, a.UnlockValue)
	if err != nil {
		return xerrors.Errorf("unable to create lock: %w", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGKILL)
	<-quit

	if err := mutex.Unlock(context.Background()); err != nil {
		return xerrors.Errorf("unable to unlock: %w", err)
	}

	return nil
}
