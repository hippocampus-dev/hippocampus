package deduplicator

import (
	"context"
	"fmt"

	"loganomaly/internal/event"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("invalid arguments: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: a.RedisAddress,
	})

	handle := func(ctx context.Context, e cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
		var data event.AnomalyEvent
		if err := e.DataAs(&data); err != nil {
			return nil, cloudevents.ResultACK
		}

		if data.Grouping == "" || data.ErrorHash == "" {
			return nil, cloudevents.ResultACK
		}

		key := fmt.Sprintf("loganomaly:%s:%s", data.Grouping, data.ErrorHash)

		set, err := redisClient.SetNX(ctx, key, "1", a.DedupTTL).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to set redis key: %w", err)
		}

		if !set {
			return nil, cloudevents.ResultACK
		}

		return &e, cloudevents.ResultACK
	}

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		return xerrors.Errorf("failed to create client: %w", err)
	}

	if err := c.StartReceiver(context.Background(), handle); err != nil {
		return xerrors.Errorf("failed to start receiver: %w", err)
	}

	return nil
}
