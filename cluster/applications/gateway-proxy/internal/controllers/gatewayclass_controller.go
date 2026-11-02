package controllers

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type GatewayClassController struct {
	client.Client
	Scheme         *runtime.Scheme
	ControllerName gatewayv1.GatewayController
}

func (r *GatewayClassController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gatewayClass := &gatewayv1.GatewayClass{}
	if err := r.Get(ctx, req.NamespacedName, gatewayClass); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if gatewayClass.Spec.ControllerName != r.ControllerName {
		return ctrl.Result{}, nil
	}

	acceptedOK := false
	supportedVersionOK := false
	for _, c := range gatewayClass.Status.Conditions {
		if c.ObservedGeneration != gatewayClass.Generation || c.Status != metav1.ConditionTrue {
			continue
		}
		switch c.Type {
		case string(gatewayv1.GatewayClassConditionStatusAccepted):
			acceptedOK = true
		case string(gatewayv1.GatewayClassConditionStatusSupportedVersion):
			supportedVersionOK = true
		}
	}
	if acceptedOK && supportedVersionOK {
		return ctrl.Result{}, nil
	}

	now := metav1.Now()
	gatewayClass.Status.Conditions = []metav1.Condition{
		{
			Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: gatewayClass.Generation,
			LastTransitionTime: now,
			Reason:             string(gatewayv1.GatewayClassReasonAccepted),
		},
		{
			Type:               string(gatewayv1.GatewayClassConditionStatusSupportedVersion),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: gatewayClass.Generation,
			LastTransitionTime: now,
			Reason:             string(gatewayv1.GatewayClassReasonSupportedVersion),
		},
	}

	if err := r.Status().Update(ctx, gatewayClass); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GatewayClassController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.GatewayClass{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
