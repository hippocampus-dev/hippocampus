package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strconv"
	"strings"
	"time"
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
			Endpoints: a.EtcdEndpoints,
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

	webhookServer := m.GetWebhookServer()
	webhookServer.Register("/mutate", &webhook.Admission{Handler: &handler{
		client:                  m.GetClient(),
		decoder:                 admission.NewDecoder(m.GetScheme()),
		sidecarImage:            a.SidecarImage,
		sidecarArgs:             a.Args.Strings(),
		enableSidecarContainers: a.EnableSidecarContainers,
		lockFactory:             lockFactory,
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
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	switch req.Kind {
	case metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}:
		pod := &corev1.Pod{}
		if err := h.decoder.Decode(req, pod); err != nil {
			handlerLogger.Error(err, "unable to decode request")
			return admission.Errored(http.StatusBadRequest, err)
		}

		if pod.Annotations["exactly-one-pod-hook.kaidotio.github.io/exactly-one-pod"] != "true" {
			return admission.Allowed("")
		}

		key := pod.Annotations["exactly-one-pod-hook.kaidotio.github.io/key"]
		if key == "" {
			return admission.Denied("valid key is not set")
		}

		expiration, err := strconv.Atoi(pod.Annotations["exactly-one-pod-hook.kaidotio.github.io/expiration"])
		if err != nil {
			return admission.Denied("valid expiration is not set")
		}

		namespacedKey := fmt.Sprintf("%s/%s", req.Namespace, key)
		mutex, err := h.lockFactory(namespacedKey, time.Duration(expiration)*time.Second)
		if err != nil {
			return admission.Denied("unable to create lock")
		}

		fencingToken, err := mutex.Lock(context.Background())
		if err != nil {
			if errors.Is(err, lock.ErrLockAlreadyTaken) {
				return admission.Denied("lock already taken")
			}
			handlerLogger.Error(err, "unable to acquire lock")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if fencingToken != nil {
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations["exactly-one-pod-hook.kaidotio.github.io/fencing-token"] = strconv.FormatInt(*fencingToken, 10)
		}

		args := []string{
			"sidecar",
			namespacedKey,
			mutex.Value(),
		}
		args = append(args, h.sidecarArgs...)
		if h.enableSidecarContainers {
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
				Name:            "exactly-one-pod-sidecar",
				Image:           h.sidecarImage,
				ImagePullPolicy: corev1.PullAlways,
				RestartPolicy:   func(p corev1.ContainerRestartPolicy) *corev1.ContainerRestartPolicy { return &p }(corev1.ContainerRestartPolicyAlways),
				Args:            args,
			})
		} else {
			pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
				Name:            "exactly-one-pod-sidecar",
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
