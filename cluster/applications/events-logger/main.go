package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/go-logr/zapr"
	"github.com/google/uuid"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	v1Informers "k8s.io/client-go/informers/events/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v1Listers "k8s.io/client-go/listers/events/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	controllerAgentName    = "events-logger"
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

type Controller struct {
	clientset kubernetes.Interface
	lister    v1Listers.EventLister
	synced    cache.InformerSynced
	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}

func NewController(clientset kubernetes.Interface, informer v1Informers.EventInformer) *Controller {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		clientset: clientset,
		lister:    informer.Lister(),
		synced:    informer.Informer().HasSynced,
		workqueue: workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{
			Name: "Events",
		}),
		recorder: recorder,
	}

	eventHandler := func(object interface{}) {
		key, err := cache.MetaNamespaceKeyFunc(object)
		if err != nil {
			utilruntime.HandleError(err)
			return
		}
		controller.workqueue.Add(key)
	}
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: eventHandler,
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			if oldObj != newObj {
				eventHandler(newObj)
			}
		},
		DeleteFunc: eventHandler,
	})

	return controller
}

func (c *Controller) Run(concurrency int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.synced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < concurrency; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh

	return nil
}

func (c *Controller) runWorker() {
	for {
		object, shutdown := c.workqueue.Get()

		if shutdown {
			return
		}

		err := func(object interface{}) error {
			defer c.workqueue.Done(object)
			key, ok := object.(string)
			if !ok {
				c.workqueue.Forget(object)
				return fmt.Errorf("expected string in workqueue but got %#v", object)
			}
			if err := c.syncHandler(key); err != nil {
				c.workqueue.AddRateLimited(key)
				return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
			}
			c.workqueue.Forget(object)
			return nil
		}(object)

		if err != nil {
			utilruntime.HandleError(err)
		}
	}
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	event, err := c.lister.Events(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))

	return nil
}

func main() {
	var concurrency int
	flag.IntVar(&concurrency, "concurrency", envOrDefaultValue("CONCURRENCY", 1), "Concurrency of worker")
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	zapLog, err := zap.NewProduction()
	if err != nil {
		klog.Fatalf("failed to create zap logger: %+v", err)
		return
	}
	zapLogger := zapr.NewLogger(zapLog)
	klog.SetLogger(zapLogger)

	stopCh := make(chan struct{}, 1)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	go func() {
		<-quit
		close(stopCh)
	}()

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("failed to create kubernetes config: %s", err.Error())
		return
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("failed to create kubernetes client: %s", err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := uuid.New().String()
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name: controllerAgentName,
			Namespace: func() string {
				namespace, err := os.ReadFile(inClusterNamespacePath)
				if err != nil {
					klog.Fatalf("failed to find leader election namespace: %+v", err)
				}
				return string(namespace)
			}(),
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*10)
				controller := NewController(clientset, informerFactory.Events().V1().Events())
				informerFactory.Start(stopCh)

				if err := controller.Run(concurrency, stopCh); err != nil {
					klog.Fatalf("failed to run controller: %s", err.Error())
					return
				}
			},
			OnStoppedLeading: func() {
				klog.Infof("leader lost: %s", id)
				os.Exit(0)
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				klog.Infof("new leader elected: %s", identity)
			},
		},
	})
}
