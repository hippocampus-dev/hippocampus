package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	grafanaV1 "grafana-manifest-controller/api/v1"
	"grafana-manifest-controller/internal/grafana"
	"grafana-manifest-controller/internal/jsonnet"

	"github.com/go-logr/logr"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	var dashboard grafanaV1.Dashboard
	if err := r.Get(ctx, req.NamespacedName, &dashboard); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	grafanaClient := grafana.NewClient(r.GrafanaURL)

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

	dashboardJSON, err := r.resolveDashboardContent(ctx, &dashboard)
	if err != nil {
		r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "ResolveError", err.Error())
		r.Recorder.Event(&dashboard, coreV1.EventTypeWarning, "ResolveFailed", err.Error())
		if statusErr := r.Status().Update(ctx, &dashboard); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	if dashboardJSON == nil {
		r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "InvalidSpec", "one of json, jsonnet, or configMapRef must be specified")
		if statusErr := r.Status().Update(ctx, &dashboard); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
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

	if dashboard.Status.UID != "" {
		currentDashboard, err := grafanaClient.GetDashboard(ctx, dashboard.Status.UID)
		if err != nil {
			logger.Error(err, "failed to get current dashboard from Grafana")
		} else if currentDashboard != nil && dashboardsEqual(dashboardJSON, currentDashboard) {
			logger.V(1).Info("dashboard unchanged, skipping sync")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
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

	if dashboard.Spec.HomeDashboard {
		prefs, err := grafanaClient.GetOrgPreferences(ctx)
		if err != nil {
			r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "HomeDashboardError", err.Error())
			r.Recorder.Event(&dashboard, coreV1.EventTypeWarning, "HomeDashboardFailed", err.Error())
			if err := r.Status().Update(ctx, &dashboard); err != nil {
				logger.Error(err, "failed to update status")
			}
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		if prefs.HomeDashboardUID != "" && prefs.HomeDashboardUID != resp.UID {
			errMsg := "another dashboard is already set as home: " + prefs.HomeDashboardUID
			r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "HomeDashboardConflict", errMsg)
			r.Recorder.Event(&dashboard, coreV1.EventTypeWarning, "HomeDashboardConflict", errMsg)
			if err := r.Status().Update(ctx, &dashboard); err != nil {
				logger.Error(err, "failed to update status")
			}
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		if prefs.HomeDashboardUID != resp.UID {
			if err := grafanaClient.SetHomeDashboard(ctx, resp.UID); err != nil {
				r.setCondition(&dashboard, "Ready", metaV1.ConditionFalse, "HomeDashboardError", err.Error())
				r.Recorder.Event(&dashboard, coreV1.EventTypeWarning, "HomeDashboardFailed", err.Error())
				if err := r.Status().Update(ctx, &dashboard); err != nil {
					logger.Error(err, "failed to update status")
				}
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}
			r.Recorder.Event(&dashboard, coreV1.EventTypeNormal, "HomeDashboardSet", "Dashboard set as home")
			logger.Info("dashboard set as home", "uid", resp.UID)
		}
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

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *DashboardReconciler) resolveDashboardContent(ctx context.Context, dashboard *grafanaV1.Dashboard) ([]byte, error) {
	var content string

	switch {
	case dashboard.Spec.Jsonnet != "":
		content = dashboard.Spec.Jsonnet
	case dashboard.Spec.ConfigMapRef != nil:
		ref := dashboard.Spec.ConfigMapRef
		var configMap coreV1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{
			Name:      ref.Name,
			Namespace: dashboard.Namespace,
		}, &configMap); err != nil {
			return nil, err
		}
		data, ok := configMap.Data[ref.Key]
		if !ok {
			return nil, apierrors.NewNotFound(coreV1.Resource("configmap"), ref.Name+"/"+ref.Key)
		}
		content = data
	default:
		return nil, nil
	}

	return r.Renderer.Render(content)
}

func (r *DashboardReconciler) setCondition(dashboard *grafanaV1.Dashboard, condType string, status metaV1.ConditionStatus, reason, message string) {
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
		For(&grafanaV1.Dashboard{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

func normalizeDashboardJSON(data []byte) ([]byte, error) {
	var dashboard map[string]interface{}
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return nil, err
	}

	delete(dashboard, "id")
	delete(dashboard, "version")

	return json.Marshal(dashboard)
}

func dashboardsEqual(desired []byte, current []byte) bool {
	normalizedDesired, err := normalizeDashboardJSON(desired)
	if err != nil {
		return false
	}

	normalizedCurrent, err := normalizeDashboardJSON(current)
	if err != nil {
		return false
	}

	return bytes.Equal(normalizedDesired, normalizedCurrent)
}
