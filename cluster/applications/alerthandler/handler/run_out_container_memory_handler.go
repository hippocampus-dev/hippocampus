package handler

import (
	"context"
	"log"
	"time"

	"golang.org/x/xerrors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type RunOutContainerMemoryHandler struct {
	client                kubernetes.Interface
	waitForDeleteInterval time.Duration
	waitForDeleteTimeout  time.Duration
}

func NewRunOutContainerMemoryHandler(client kubernetes.Interface, interval time.Duration, timeout time.Duration) *RunOutContainerMemoryHandler {
	return &RunOutContainerMemoryHandler{
		client:                client,
		waitForDeleteInterval: interval,
		waitForDeleteTimeout:  timeout,
	}
}

func (handler *RunOutContainerMemoryHandler) Call(request *AlertManagerRequest) error {
	ctx := context.Background()
	for _, alert := range request.Alerts {
		namespace := alert.Labels["namespace"]
		name := alert.Labels["pod"]
		if namespace == "" || name == "" {
			return xerrors.New("alert must have namespace,pod label")
		}
		pdbList, err := handler.client.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metaV1.ListOptions{})
		if err != nil {
			return xerrors.Errorf("failed to list pdb in %s: %w", namespace, err)
		}
		for _, pdb := range pdbList.Items {
			podList, err := handler.client.CoreV1().Pods(namespace).List(ctx, metaV1.ListOptions{
				LabelSelector: labels.Set(pdb.Spec.Selector.MatchLabels).String(),
			})
			if err != nil {
				return xerrors.Errorf("failed to list pod in %s: %w", namespace, err)
			}
			for _, pod := range podList.Items {
				if pod.Namespace == namespace && pod.Name == name {
					if err := handler.waitForDelete(ctx, pdb.Namespace, pdb.Name); err != nil {
						return xerrors.Errorf("failed to waitForDelete %s/%s: %w", name, namespace, err)
					}
					if err := handler.client.CoreV1().Pods(namespace).Delete(ctx, name, metaV1.DeleteOptions{}); err != nil {
						return xerrors.Errorf("failed to delete pod %s/%s: %w", name, namespace, err)
					}
					log.Printf("Successful to delete pod %s/%s", name, namespace)
				}
			}
		}
	}
	return nil
}
func (handler *RunOutContainerMemoryHandler) waitForDelete(ctx context.Context, namespace string, name string) error {
	if err := wait.PollUntilContextTimeout(ctx, handler.waitForDeleteInterval, handler.waitForDeleteTimeout, true, func(ctx context.Context) (bool, error) {
		pdb, err := handler.client.PolicyV1().PodDisruptionBudgets(namespace).Get(ctx, name, metaV1.GetOptions{})
		if err != nil {
			return false, xerrors.Errorf("failed to get pdb in %s/%s: %w", name, namespace, err)
		}
		return pdb.Status.DisruptionsAllowed > 0, nil
	}); err != nil {
		return xerrors.Errorf("failed to wait PodDisruptionBudget: %w", err)
	}
	return nil
}
