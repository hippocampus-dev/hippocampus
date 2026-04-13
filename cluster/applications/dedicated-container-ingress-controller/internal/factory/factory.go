package factory

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const entriesKey = "entries"

var ErrNoBackend = errors.New("no backend found")

type DedicatedContainerFactory struct {
	redisClient *redis.Client
	clientset   kubernetes.Interface
}

func NewDedicatedContainerFactory(redisClient *redis.Client, clientset kubernetes.Interface) *DedicatedContainerFactory {
	return &DedicatedContainerFactory{
		redisClient: redisClient,
		clientset:   clientset,
	}
}

func (f *DedicatedContainerFactory) StoreEntry(ctx context.Context, host string, podTemplateSpec corev1.PodTemplateSpec) error {
	data, err := json.Marshal(podTemplateSpec)
	if err != nil {
		return xerrors.Errorf("failed to marshal PodTemplateSpec: %w", err)
	}
	if err := f.redisClient.HSet(ctx, entriesKey, host, data).Err(); err != nil {
		return xerrors.Errorf("failed to HSET: %w", err)
	}
	return nil
}

func (f *DedicatedContainerFactory) DeleteEntry(ctx context.Context, host string) error {
	if err := f.redisClient.HDel(ctx, entriesKey, host).Err(); err != nil {
		return xerrors.Errorf("failed to HDEL: %w", err)
	}
	return nil
}

func (f *DedicatedContainerFactory) HasEntry(ctx context.Context, host string) (bool, error) {
	exists, err := f.redisClient.HExists(ctx, entriesKey, host).Result()
	if err != nil {
		return false, xerrors.Errorf("failed to HEXISTS: %w", err)
	}
	return exists, nil
}

func (f *DedicatedContainerFactory) Create(ctx context.Context, host string) (*corev1.Pod, error) {
	data, err := f.redisClient.HGet(ctx, entriesKey, host).Result()
	if errors.Is(err, redis.Nil) {
		return nil, xerrors.Errorf("%w: %s", ErrNoBackend, host)
	}
	if err != nil {
		return nil, xerrors.Errorf("failed to HGET: %w", err)
	}
	var podTemplateSpec corev1.PodTemplateSpec
	if err := json.Unmarshal([]byte(data), &podTemplateSpec); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal PodTemplateSpec: %w", err)
	}

	objectMeta := podTemplateSpec.ObjectMeta
	objectMeta.GenerateName = strings.ReplaceAll(host, ".", "-") + "-"
	labels := map[string]string{
		"owner": "dedicated-container-ingress-controller",
	}
	for k, v := range objectMeta.Labels {
		labels[k] = v
	}
	objectMeta.Labels = labels
	createdPod, err := f.clientset.CoreV1().Pods(podTemplateSpec.Namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: objectMeta,
		Spec:       podTemplateSpec.Spec,
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, xerrors.Errorf("failed to create pod: %w", err)
	}

	var pod *corev1.Pod
	if err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err = f.clientset.CoreV1().Pods(podTemplateSpec.Namespace).Get(ctx, createdPod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodRunning, nil
	}); err != nil {
		_ = f.clientset.CoreV1().Pods(podTemplateSpec.Namespace).Delete(ctx, createdPod.Name, metav1.DeleteOptions{})
		return nil, xerrors.Errorf("failed to wait for pod to become running: %w", err)
	}

	return pod, nil
}
