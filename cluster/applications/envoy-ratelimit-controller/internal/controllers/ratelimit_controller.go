package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "envoy-ratelimit-controller/api/v1"
	"envoy-ratelimit-controller/internal/ratelimit"
	"envoy-ratelimit-controller/internal/xds"

	rlsConfV3 "github.com/envoyproxy/go-control-plane/ratelimit/config/ratelimit/v3"
	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type RateLimitReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	Recorder           record.EventRecorder
	Server             *xds.Server
	IstioRootNamespace string
}

func (r *RateLimitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("ratelimit", req.NamespacedName)

	var rateLimit v1.RateLimit
	if err := r.Get(ctx, req.NamespacedName, &rateLimit); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.Server.Delete(ctx, req.NamespacedName)
		}
		return ctrl.Result{}, err
	}

	domain := fmt.Sprintf("%s.%s", rateLimit.Namespace, rateLimit.Name)

	// Nothing is published until the spec passes these checks, so the last accepted config and EnvoyFilter stay in effect
	config, envoyFilter, reason, err := r.generate(ctx, &rateLimit, domain)
	if err != nil {
		r.setNotReady(ctx, logger, &rateLimit, reason, err)
		if reason == "EnvoyFilterFailed" {
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		return ctrl.Result{}, nil
	}

	if err := r.Server.Set(ctx, req.NamespacedName, config); err != nil {
		r.setNotReady(ctx, logger, &rateLimit, "PublishFailed", err)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if err := r.reconcileEnvoyFilter(ctx, &rateLimit, envoyFilter, false); err != nil {
		r.setNotReady(ctx, logger, &rateLimit, "EnvoyFilterFailed", err)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	status, reason, message := r.Server.Status(req.NamespacedName)
	changed := meta.SetStatusCondition(&rateLimit.Status.Conditions, metaV1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: rateLimit.Generation,
	})
	if !changed && rateLimit.Status.Domain == domain {
		return ctrl.Result{}, nil
	}
	rateLimit.Status.Domain = domain
	if changed && status == metaV1.ConditionTrue {
		now := metaV1.Now()
		rateLimit.Status.LastSyncedAt = &now
	}

	if err := r.Status().Update(ctx, &rateLimit); err != nil {
		return ctrl.Result{}, err
	}

	if changed && status == metaV1.ConditionTrue {
		r.Recorder.Event(&rateLimit, coreV1.EventTypeNormal, "Synced", "RateLimit config accepted by envoy-ratelimit")
	}

	return ctrl.Result{}, nil
}

func (r *RateLimitReconciler) generate(ctx context.Context, rateLimit *v1.RateLimit, domain string) (*rlsConfV3.RateLimitConfig, *unstructured.Unstructured, string, error) {
	if rateLimit.Namespace == r.IstioRootNamespace {
		return nil, nil, "RootNamespace", xerrors.New("RateLimit in the Istio root namespace is not supported")
	}

	config, err := ratelimit.GenerateConfig(domain, rateLimit.Spec.Descriptors)
	if err != nil {
		return nil, nil, "InvalidDescriptors", err
	}

	envoyFilter, err := ratelimit.GenerateEnvoyFilter(rateLimit, domain)
	if err != nil {
		return nil, nil, "InvalidFilter", err
	}

	// The Istio validating webhook rejects mistyped filter fields only when the EnvoyFilter is written
	if err := r.reconcileEnvoyFilter(ctx, rateLimit, envoyFilter.DeepCopy(), true); err != nil {
		return nil, nil, "EnvoyFilterFailed", err
	}

	return config, envoyFilter, "", nil
}

func (r *RateLimitReconciler) LoadAll(ctx context.Context) error {
	var rateLimits v1.RateLimitList
	if err := r.List(ctx, &rateLimits); err != nil {
		return xerrors.Errorf("failed to list RateLimits: %w", err)
	}

	for i := range rateLimits.Items {
		rateLimit := &rateLimits.Items[i]
		if rateLimit.Namespace == r.IstioRootNamespace {
			continue
		}
		domain := fmt.Sprintf("%s.%s", rateLimit.Namespace, rateLimit.Name)
		// No EnvoyFilter dry-run here, since a transient webhook failure would otherwise drop a valid config from the first snapshot
		config, err := ratelimit.GenerateConfig(domain, rateLimit.Spec.Descriptors)
		if err != nil {
			continue
		}
		if err := r.Server.Set(ctx, client.ObjectKeyFromObject(rateLimit), config); err != nil {
			return err
		}
	}

	return r.Server.Initialize(ctx)
}

func (r *RateLimitReconciler) setNotReady(ctx context.Context, logger logr.Logger, rateLimit *v1.RateLimit, reason string, err error) {
	meta.SetStatusCondition(&rateLimit.Status.Conditions, metaV1.Condition{
		Type:               "Ready",
		Status:             metaV1.ConditionFalse,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: rateLimit.Generation,
	})
	if statusErr := r.Status().Update(ctx, rateLimit); statusErr != nil {
		logger.Error(statusErr, "failed to update status")
	}
}

func (r *RateLimitReconciler) reconcileEnvoyFilter(ctx context.Context, rateLimit *v1.RateLimit, desired *unstructured.Unstructured, dryRun bool) error {
	if err := controllerutil.SetControllerReference(rateLimit, desired, r.Scheme); err != nil {
		return xerrors.Errorf("failed to set controller reference: %w", err)
	}

	var createOptions []client.CreateOption
	var updateOptions []client.UpdateOption
	if dryRun {
		createOptions = append(createOptions, client.DryRunAll)
		updateOptions = append(updateOptions, client.DryRunAll)
	}

	var existing unstructured.Unstructured
	existing.SetGroupVersionKind(desired.GroupVersionKind())

	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.GetName(),
		Namespace: desired.GetNamespace(),
	}, &existing)

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired, createOptions...); err != nil {
			return xerrors.Errorf("failed to create EnvoyFilter: %w", err)
		}
		return nil
	}
	if err != nil {
		return xerrors.Errorf("failed to get EnvoyFilter: %w", err)
	}
	if !metaV1.IsControlledBy(&existing, rateLimit) {
		return xerrors.Errorf("EnvoyFilter %s/%s is not owned by this RateLimit", existing.GetNamespace(), existing.GetName())
	}

	desired.SetResourceVersion(existing.GetResourceVersion())
	if err := r.Update(ctx, desired, updateOptions...); err != nil {
		return xerrors.Errorf("failed to update EnvoyFilter: %w", err)
	}

	return nil
}

func (r *RateLimitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var envoyFilter unstructured.Unstructured
	envoyFilter.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1alpha3",
		Kind:    "EnvoyFilter",
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.RateLimit{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&envoyFilter, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WatchesRawSource(source.Channel(r.Server.Events, &handler.EnqueueRequestForObject{})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
