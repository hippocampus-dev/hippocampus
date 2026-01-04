package controllers

import (
	"context"
	"time"

	grafanaV1 "grafana-manifest-controller/api/v1"
	"grafana-manifest-controller/internal/grafana"
	"grafana-manifest-controller/internal/jsonnet"

	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	finalizerName = "grafana-manifest.kaidotio.github.io/finalizer"
)

type DashboardReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	Renderer   *jsonnet.Renderer
	GrafanaURL string
}

func (r *DashboardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("dashboard", req.NamespacedName)

	var dashboard grafanaV1.GrafanaDashboard
	if err := r.Get(ctx, req.NamespacedName, &dashboard); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	grafanaClient, err := r.getGrafanaClient(ctx, &dashboard)
	if err != nil {
		r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "SecretError", err.Error())
		if err := r.Status().Update(ctx, &dashboard); err != nil {
			logger.Error(err, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if !dashboard.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&dashboard, finalizerName) {
			if dashboard.Status.UID != "" {
				if err := grafanaClient.DeleteDashboard(ctx, dashboard.Status.UID); err != nil {
					logger.Error(err, "failed to delete dashboard from Grafana")
					return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
				}
				r.Recorder.Event(&dashboard, coreV1.EventTypeNormal, "Deleted", "Dashboard deleted from Grafana")
			}

			controllerutil.RemoveFinalizer(&dashboard, finalizerName)
			if err := r.Update(ctx, &dashboard); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&dashboard, finalizerName) {
		controllerutil.AddFinalizer(&dashboard, finalizerName)
		if err := r.Update(ctx, &dashboard); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	var dashboardJSON []byte
	if dashboard.Spec.Jsonnet != "" {
		dashboardJSON, err = r.Renderer.Render(dashboard.Spec.Jsonnet)
		if err != nil {
			r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "RenderError", err.Error())
			r.Recorder.Event(&dashboard, coreV1.EventTypeWarning, "RenderFailed", err.Error())
			if err := r.Status().Update(ctx, &dashboard); err != nil {
				logger.Error(err, "failed to update status")
			}
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	} else if dashboard.Spec.JSON != "" {
		dashboardJSON = []byte(dashboard.Spec.JSON)
	} else {
		r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "InvalidSpec", "either jsonnet or json must be specified")
		if err := r.Status().Update(ctx, &dashboard); err != nil {
			logger.Error(err, "failed to update status")
		}
		return ctrl.Result{}, nil
	}

	folderUID, err := grafanaClient.EnsureFolder(ctx, dashboard.Spec.Folder)
	if err != nil {
		r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "FolderError", err.Error())
		if err := r.Status().Update(ctx, &dashboard); err != nil {
			logger.Error(err, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	resp, err := grafanaClient.UpsertDashboard(ctx, dashboardJSON, folderUID)
	if err != nil {
		r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "SyncError", err.Error())
		r.Recorder.Event(&dashboard, coreV1.EventTypeWarning, "SyncFailed", err.Error())
		if err := r.Status().Update(ctx, &dashboard); err != nil {
			logger.Error(err, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	now := metaV1.Now()
	dashboard.Status.UID = resp.UID
	dashboard.Status.URL = resp.URL
	dashboard.Status.Version = resp.Version
	dashboard.Status.LastSyncedAt = &now
	r.setCondition(&dashboard, "Ready", metaV1.ConditionTrue, "Synced", "Dashboard synced to Grafana")

	if err := r.Status().Update(ctx, &dashboard); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Eventf(&dashboard, coreV1.EventTypeNormal, "Synced", "Dashboard synced to Grafana: %s", resp.URL)
	logger.Info("dashboard synced", "uid", resp.UID, "version", resp.Version)

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *DashboardReconciler) getGrafanaClient(ctx context.Context, dashboard *grafanaV1.GrafanaDashboard) (*grafana.Client, error) {
	var secret coreV1.Secret
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: dashboard.Namespace,
		Name:      dashboard.Spec.GrafanaRef.SecretRef.Name,
	}, &secret); err != nil {
		return nil, xerrors.Errorf("failed to get secret: %w", err)
	}

	apiKey := string(secret.Data["api-key"])
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	return grafana.NewClient(r.GrafanaURL, apiKey, username, password), nil
}

func (r *DashboardReconciler) setCondition(dashboard *grafanaV1.GrafanaDashboard, condType string, status metaV1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&dashboard.Status.Conditions, metaV1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metaV1.Now(),
	})
}

func (r *DashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&grafanaV1.GrafanaDashboard{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
