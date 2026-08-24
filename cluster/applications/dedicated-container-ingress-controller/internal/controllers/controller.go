package controllers

import (
	"context"

	ingressv1 "dedicated-container-ingress-controller/api/v1"
	"dedicated-container-ingress-controller/internal/factory"

	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const finalizerName = "dedicated-container-ingress.kaidotio.github.io/finalizer"

type DedicatedContainerIngressReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	Recorder record.EventRecorder
	Factory  *factory.DedicatedContainerFactory
}

func (r *DedicatedContainerIngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ingress := &ingressv1.DedicatedContainerIngress{}
	if err := r.Client.Get(ctx, req.NamespacedName, ingress); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, xerrors.Errorf("failed to get DedicatedContainerIngress: %w", err)
	}

	host := ingress.Spec.Host

	if ingress.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(ingress, finalizerName) {
			controllerutil.AddFinalizer(ingress, finalizerName)
			if err := r.Client.Update(ctx, ingress); err != nil {
				return ctrl.Result{}, xerrors.Errorf("failed to add finalizer: %w", err)
			}
		}

		exists, err := r.Factory.HasEntry(ctx, host)
		if err != nil {
			return ctrl.Result{}, xerrors.Errorf("failed to check entry: %w", err)
		}
		podTemplateSpec := corev1.PodTemplateSpec{
			ObjectMeta: ingress.Spec.Template.ObjectMeta,
			Spec:       ingress.Spec.Template.Spec,
		}
		podTemplateSpec.Namespace = ingress.Namespace
		if err := r.Factory.StoreEntry(ctx, host, podTemplateSpec); err != nil {
			return ctrl.Result{}, xerrors.Errorf("failed to store entry: %w", err)
		}
		if !exists {
			r.Recorder.Eventf(ingress, corev1.EventTypeNormal, "SuccessfulCreated", "Created entry: %q", host)
		} else {
			r.Recorder.Eventf(ingress, corev1.EventTypeNormal, "SuccessfulUpdated", "Updated entry: %q", host)
		}
	} else if controllerutil.ContainsFinalizer(ingress, finalizerName) {
		if err := r.Factory.DeleteEntry(ctx, host); err != nil {
			return ctrl.Result{}, xerrors.Errorf("failed to delete entry: %w", err)
		}
		r.Recorder.Eventf(ingress, corev1.EventTypeNormal, "SuccessfulDeleted", "Deleted entry: %q", host)

		controllerutil.RemoveFinalizer(ingress, finalizerName)
		if err := r.Client.Update(ctx, ingress); err != nil {
			return ctrl.Result{}, xerrors.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *DedicatedContainerIngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ingressv1.DedicatedContainerIngress{}).
		Complete(r)
}
