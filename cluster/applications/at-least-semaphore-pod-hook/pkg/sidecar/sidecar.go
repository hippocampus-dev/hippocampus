package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"golang.org/x/xerrors"
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

	queueRedisClient := redis.NewClient(&redis.Options{
		Addr: a.QueueRedisAddress,
	})

	inflightKey := fmt.Sprintf("heartbeat/%s", a.QueueName)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGKILL)

	queueRedisClient.ZAdd(context.Background(), inflightKey, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: a.QueueValue,
	})

	ticker := time.NewTicker(time.Duration(a.HeartbeatIntervalSeconds) * time.Second)
	defer ticker.Stop()

	func() {
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				queueRedisClient.ZAdd(context.Background(), inflightKey, redis.Z{
					Score:  float64(time.Now().Unix()),
					Member: a.QueueValue,
				})
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.TerminationGracePeriodSeconds)*time.Second)
	defer cancel()

	var unlockErr error
	for {
		select {
		case <-ctx.Done():
			if unlockErr != nil {
				return xerrors.Errorf("unable to unlock: %w", unlockErr)
			}
			return nil
		default:
			if err := queueRedisClient.ZRem(ctx, inflightKey, a.QueueValue).Err(); err != nil {
				unlockErr = err
				time.Sleep(1 * time.Second)
				continue
			}
			if err := queueRedisClient.RPush(ctx, fmt.Sprintf("queue/%s", a.QueueName), a.QueueValue).Err(); err != nil {
				unlockErr = err
				time.Sleep(1 * time.Second)
				continue
			}
			return nil
		}
	}
}
