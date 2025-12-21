package webhook

import (
	"at-least-semaphore-pod-hook/internal/lock"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redsync/redsync/v4"
	redsyncredis "github.com/go-redsync/redsync/v4/redis"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func apiGroup() string {
	defaultGroup := "at-least-semaphore-pod-hook.kaidotio.github.io"
	if v, ok := os.LookupEnv("VARIANT"); ok {
		return fmt.Sprintf("%s.%s", v, defaultGroup)
	}
	return defaultGroup
}

var (
	queueGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "at_least_semaphore_queue",
			Help: "Number of items in the queue",
		},
	)
	inflightGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "at_least_semaphore_inflight",
			Help: "Number of items in flight",
		},
	)
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

	entrypointLogger := ctrl.Log.WithName("entrypoint")

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		entrypointLogger.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !a.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	metrics.Registry.MustRegister(queueGauge, inflightGauge)

	m, err := ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress:   a.MetricsAddr,
			SecureServing: a.SecureMetrics,
			TLSOpts:       tlsOpts,
		},
		HealthProbeBindAddress: a.ProbeAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    a.Host,
			Port:    a.Port,
			CertDir: a.CertDir,
		}),
	})
	if err != nil {
		return xerrors.Errorf("unable to create manager: %w", err)
	}

	var lockFactory func(key string, expiration time.Duration) (lock.Lock, error)

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

		lockFactory = func(key string, expiration time.Duration) (lock.Lock, error) {
			return &lock.RedsyncWrapper{Mutex: redlock.NewMutex(key, redsync.WithExpiry(expiration))}, nil
		}
	case "etcd":
		etcdClient, err := clientv3.New(clientv3.Config{
			Endpoints: a.EtcdAddresses,
		})
		if err != nil {
			return xerrors.Errorf("unable to create etcd client: %w", err)
		}
		defer etcdClient.Close()

		lockFactory = func(key string, expiration time.Duration) (lock.Lock, error) {
			session, err := concurrency.NewSession(etcdClient, concurrency.WithTTL(int(expiration.Seconds())))
			if err != nil {
				return nil, xerrors.Errorf("unable to create etcd session: %w", err)
			}
			session.Orphan()

			return &lock.EtcdWrapper{Mutex: concurrency.NewMutex(session, key)}, nil
		}
	default:
		return xerrors.Errorf("invalid lock mode: %s", a.LockMode)
	}

	queueRedisClient := redis.NewClient(&redis.Options{
		Addr: a.QueueRedisAddress,
	})

	webhookServer := m.GetWebhookServer()
	webhookServer.Register("/mutate", &webhook.Admission{Handler: &handler{
		client:                  m.GetClient(),
		decoder:                 admission.NewDecoder(m.GetScheme()),
		sidecarImage:            a.SidecarImage,
		sidecarArgs:             a.Args.Strings(),
		enableSidecarContainers: a.EnableSidecarContainers,
		lockFactory:             lockFactory,
		queueRedisClient:        queueRedisClient,
	}})

	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return xerrors.Errorf("unable to set up health check: %w", err)
	}
	if err := m.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return xerrors.Errorf("unable to set up ready check: %w", err)
	}

	entrypointLogger.Info("starting manager")
	if err := m.Start(ctrl.SetupSignalHandler()); err != nil {
		return xerrors.Errorf("unable to run manager: %w", err)
	}

	return nil
}

type handler struct {
	client                  client.Client
	decoder                 *admission.Decoder
	sidecarImage            string
	sidecarArgs             []string
	enableSidecarContainers bool
	lockFactory             func(key string, expiration time.Duration) (lock.Lock, error)
	queueRedisClient        *redis.Client
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	switch req.Kind {
	case metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}:
		pod := &corev1.Pod{}
		if err := h.decoder.DecodeRaw(req.Object, pod); err != nil {
			handlerLogger.Error(err, "unable to decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		if pod.Annotations[fmt.Sprintf("%s/at-least-semaphore-pod", apiGroup())] != "true" {
			return admission.Allowed("")
		}

		key := pod.Annotations[fmt.Sprintf("%s/key", apiGroup())]
		if key == "" {
			return admission.Denied("valid key is not set")
		}

		length := pod.Annotations[fmt.Sprintf("%s/length", apiGroup())]
		if length == "" {
			return admission.Denied("valid length is not set")
		}

		expiration, err := strconv.Atoi(pod.Annotations[fmt.Sprintf("%s/expiration", apiGroup())])
		if err != nil {
			return admission.Denied("valid expiration is not set")
		}

		namespacedKey := fmt.Sprintf("%s/%s", req.Namespace, key)
		mutex, err := h.lockFactory(namespacedKey, time.Duration(expiration)*time.Second)
		if err != nil {
			return admission.Denied("unable to create lock")
		}

		if _, err := mutex.Lock(ctx); err != nil {
			if errors.Is(err, lock.ErrLockAlreadyTaken) {
				return admission.Denied("lock already taken")
			}
			handlerLogger.Error(err, "unable to acquire lock")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// Initialize semaphore if it doesn't exist
		queueNamespacedKey := fmt.Sprintf("queue/%s", namespacedKey)
		inflightNamespacedKey := fmt.Sprintf("inflight/%s", namespacedKey)

		queue := h.queueRedisClient.LLen(ctx, queueNamespacedKey).Val()
		inflight := h.queueRedisClient.SCard(ctx, inflightNamespacedKey).Val()
		queueGauge.Set(float64(queue))
		inflightGauge.Set(float64(inflight))

		l, err := strconv.Atoi(length)
		if err != nil {
			if err := mutex.Unlock(ctx); err != nil {
				handlerLogger.Error(err, "unable to unlock")
				return admission.Errored(http.StatusInternalServerError, err)
			}

			handlerLogger.Error(err, "unable to convert length")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		diff := l - int(queue+inflight)

		if diff > 0 {
			pipeline := h.queueRedisClient.Pipeline()
			for i := 0; i < diff; i++ {
				if _, err := pipeline.RPush(ctx, queueNamespacedKey, i).Result(); err != nil {
					if err := mutex.Unlock(ctx); err != nil {
						handlerLogger.Error(err, "unable to unlock")
						return admission.Errored(http.StatusInternalServerError, err)
					}

					handlerLogger.Error(err, "unable to push to queue")
					return admission.Errored(http.StatusInternalServerError, err)
				}
			}

			if _, err := pipeline.Exec(ctx); err != nil {
				if err := mutex.Unlock(ctx); err != nil {
					handlerLogger.Error(err, "unable to unlock")
					return admission.Errored(http.StatusInternalServerError, err)
				}

				handlerLogger.Error(err, "unable to execute pipeline")
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}

		if err := mutex.Unlock(ctx); err != nil {
			handlerLogger.Error(err, "unable to unlock")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		value, err := h.queueRedisClient.LPop(ctx, queueNamespacedKey).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return admission.Denied("no semaphore available")
			}
			handlerLogger.Error(err, "unable to pop from queue")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if _, err := h.queueRedisClient.SAdd(ctx, inflightNamespacedKey, value).Result(); err != nil {
			handlerLogger.Error(err, "unable to push to inflight")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		args := []string{
			"sidecar",
			namespacedKey,
			value,
		}
		args = append(args, h.sidecarArgs...)
		if h.enableSidecarContainers {
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
				Name:            "at-least-semaphore-pod-sidecar",
				Image:           h.sidecarImage,
				ImagePullPolicy: corev1.PullAlways,
				RestartPolicy:   func(p corev1.ContainerRestartPolicy) *corev1.ContainerRestartPolicy { return &p }(corev1.ContainerRestartPolicyAlways),
				Args:            args,
			})
		} else {
			pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
				Name:            "at-least-semaphore-pod-sidecar",
				Image:           h.sidecarImage,
				ImagePullPolicy: corev1.PullAlways,
				Args:            args,
			})
		}

		marshalledPod, err := json.Marshal(pod)
		if err != nil {
			handlerLogger.Error(err, "unable to marshal pod")
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshalledPod)
	}

	return admission.Allowed("")
}
