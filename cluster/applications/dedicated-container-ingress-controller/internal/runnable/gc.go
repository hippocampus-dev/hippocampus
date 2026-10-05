package runnable

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

type GarbageCollector struct {
	interval    time.Duration
	lifetime    time.Duration
	redisClient *redis.Client
	clientset   kubernetes.Interface
}

func NewGarbageCollector(redisClient *redis.Client, clientset kubernetes.Interface) *GarbageCollector {
	return &GarbageCollector{
		interval:    envOrDefaultValue("GC_INTERVAL", 1*time.Minute),
		lifetime:    envOrDefaultValue("GC_LIFETIME", 1*time.Hour),
		redisClient: redisClient,
		clientset:   clientset,
	}
}

func (g *GarbageCollector) NeedLeaderElection() bool {
	return true
}

func (g *GarbageCollector) Start(ctx context.Context) error {
	gcLogger := ctrl.Log.WithName("gc")

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			g.collect(ctx, gcLogger)
		case <-ctx.Done():
			return nil
		}
	}
}

func (g *GarbageCollector) collect(ctx context.Context, logger logr.Logger) {
	threshold := fmt.Sprintf("%d", time.Now().Unix()-int64(g.lifetime/time.Second))
	identifiers, err := g.redisClient.ZRangeByScore(ctx, "pods", &redis.ZRangeBy{
		Min: "-inf",
		Max: threshold,
	}).Result()
	if err != nil {
		logger.Error(err, "failed to ZRANGEBYSCORE")
		return
	}

	for _, identifier := range identifiers {
		parts := strings.SplitN(identifier, "/", 2)
		if len(parts) != 2 {
			continue
		}
		pod, namespace := parts[0], parts[1]
		if err := g.clientset.CoreV1().Pods(namespace).Delete(ctx, pod, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete pod", "pod", pod, "namespace", namespace)
			continue
		}
		if err := g.redisClient.ZRem(ctx, "pods", identifier).Err(); err != nil {
			logger.Error(err, "failed to ZREM", "identifier", identifier)
		}
	}
}
