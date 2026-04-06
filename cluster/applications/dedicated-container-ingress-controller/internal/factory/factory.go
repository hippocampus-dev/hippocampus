package factory

import (
	"context"
	"sync"
	"time"

	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type DedicatedContainerFactory struct {
	table     sync.Map
	clientset kubernetes.Interface
}

func NewDedicatedContainerFactory(clientset kubernetes.Interface) *DedicatedContainerFactory {
	return &DedicatedContainerFactory{clientset: clientset}
}

func (f *DedicatedContainerFactory) AddEntry(host string, podTemplateSpec corev1.PodTemplateSpec) {
	f.table.Store(host, podTemplateSpec)
}

func (f *DedicatedContainerFactory) DeleteEntry(host string) {
	f.table.Delete(host)
}

func (f *DedicatedContainerFactory) HasEntry(host string) bool {
	_, ok := f.table.Load(host)
	return ok
}

func (f *DedicatedContainerFactory) Create(ctx context.Context, host string) (*corev1.Pod, error) {
	v, ok := f.table.Load(host)
	if !ok {
		return nil, xerrors.Errorf("no backend found for host %s", host)
	}
	podTemplateSpec := v.(corev1.PodTemplateSpec)
	objectMeta := podTemplateSpec.ObjectMeta
	objectMeta.GenerateName = host + "-"
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
