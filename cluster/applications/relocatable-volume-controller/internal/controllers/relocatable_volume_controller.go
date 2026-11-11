package controllers

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

func apiGroup() string {
	defaultGroup := "relocatable-volume-controller.kaidotio.github.io"
	if v, ok := os.LookupEnv("VARIANT"); ok {
		return fmt.Sprintf("%s.%s", v, defaultGroup)
	}
	return defaultGroup
}

type RelocatableVolumeController struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *RelocatableVolumeController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := &coreV1.Pod{}
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, xerrors.Errorf("failed to get pod: %w", err)
	}

	value, ok := pod.Annotations[fmt.Sprintf("%s/volumes", apiGroup())]
	if !ok {
		return ctrl.Result{}, nil
	}

	// A pod on an unreachable node stays Terminating until it is force deleted: https://kubernetes.io/docs/tasks/run-application/force-delete-stateful-set-pod/
	if pod.DeletionTimestamp != nil {
		if pod.Spec.NodeName == "" {
			return ctrl.Result{}, nil
		}
		node := &coreV1.Node{}
		if err := r.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		if ready(node) {
			return ctrl.Result{}, nil
		}
		if remaining := time.Until(pod.DeletionTimestamp.Time); remaining > 0 {
			return ctrl.Result{RequeueAfter: remaining}, nil
		}
		if err := r.Delete(ctx, pod, client.Preconditions{UID: &pod.UID}, client.GracePeriodSeconds(0)); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, xerrors.Errorf("failed to force delete pod: %w", err)
		}
		r.Recorder.Eventf(pod, coreV1.EventTypeNormal, "SuccessfulDeleted", "Node %s stayed not ready past the grace period, so the pod was force deleted", node.Name)
		return ctrl.Result{}, nil
	}

	if pod.Spec.NodeName != "" {
		return ctrl.Result{}, nil
	}

	if !slices.ContainsFunc(pod.Status.Conditions, func(condition coreV1.PodCondition) bool {
		return condition.Type == coreV1.PodScheduled && condition.Status == coreV1.ConditionFalse && condition.Reason == coreV1.PodReasonUnschedulable
	}) {
		return ctrl.Result{}, nil
	}

	relocatableVolumes := strings.Split(value, ",")

	var nodeList coreV1.NodeList
	if err := r.List(ctx, &nodeList); err != nil {
		return ctrl.Result{}, xerrors.Errorf("failed to list nodes: %w", err)
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil || !slices.Contains(relocatableVolumes, volume.Name) {
			continue
		}

		claim := &coreV1.PersistentVolumeClaim{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: volume.PersistentVolumeClaim.ClaimName}, claim); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, xerrors.Errorf("failed to get persistentvolumeclaim: %w", err)
			}
			return ctrl.Result{}, r.deletePod(ctx, pod, fmt.Sprintf("Persistentvolumeclaim %s is missing", volume.PersistentVolumeClaim.ClaimName))
		}
		if _, ok := claim.Annotations[annotation("relocating-from")]; ok {
			continue
		}
		if claim.DeletionTimestamp != nil {
			return ctrl.Result{}, r.deletePod(ctx, pod, fmt.Sprintf("Persistentvolumeclaim %s is being deleted", claim.Name))
		}
		if claim.Spec.VolumeName == "" {
			continue
		}

		volumeObject := &coreV1.PersistentVolume{}
		if err := r.Get(ctx, types.NamespacedName{Name: claim.Spec.VolumeName}, volumeObject); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return ctrl.Result{}, xerrors.Errorf("failed to get persistentvolume: %w", err)
		}
		if volumeObject.Spec.NodeAffinity == nil || volumeObject.Spec.NodeAffinity.Required == nil {
			continue
		}

		reachable := false
		for i := range nodeList.Items {
			node := &nodeList.Items[i]
			if !ready(node) {
				continue
			}
			matched, err := corev1helpers.MatchNodeSelectorTerms(node, volumeObject.Spec.NodeAffinity.Required)
			if err != nil {
				return ctrl.Result{}, xerrors.Errorf("failed to match node selector terms: %w", err)
			}
			if matched {
				reachable = true
				break
			}
		}
		if reachable {
			continue
		}

		if err := r.Delete(ctx, claim, client.Preconditions{UID: &claim.UID}); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, xerrors.Errorf("failed to delete persistentvolumeclaim: %w", err)
		}
		r.Recorder.Eventf(claim, coreV1.EventTypeNormal, "SuccessfulDeleted", "Deleted persistentvolumeclaim bound to %s, which no schedulable node matches", volumeObject.Name)
		return ctrl.Result{}, r.deletePod(ctx, pod, fmt.Sprintf("Persistentvolumeclaim %s was deleted", claim.Name))
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func ready(node *coreV1.Node) bool {
	return slices.ContainsFunc(node.Status.Conditions, func(condition coreV1.NodeCondition) bool {
		return condition.Type == coreV1.NodeReady && condition.Status == coreV1.ConditionTrue
	})
}

func (r *RelocatableVolumeController) deletePod(ctx context.Context, pod *coreV1.Pod, reason string) error {
	if err := r.Delete(ctx, pod, client.Preconditions{UID: &pod.UID}); client.IgnoreNotFound(err) != nil {
		return xerrors.Errorf("failed to delete pod: %w", err)
	}
	r.Recorder.Eventf(pod, coreV1.EventTypeNormal, "SuccessfulDeleted", "%s, so the pod was deleted", reason)
	return nil
}

func (r *RelocatableVolumeController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&coreV1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
