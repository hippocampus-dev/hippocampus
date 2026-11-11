package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	grafanaV1 "grafana-manifest-controller/api/v1"
	"grafana-manifest-controller/internal/grafana"
	"grafana-manifest-controller/internal/jsonnet"

	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	coreV1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	finalizerName       = "grafana-manifest.kaidotio.github.io/finalizer"
	grafanaPortName     = "http"
	grafanaPortFallback = int32(3000)
)

type RenderCacheEntry struct {
	Generation               int64
	ConfigMapResourceVersion string
	RenderedJSON             []byte
}

type DashboardReconciler struct {
	client.Client
	Log         logr.Logger
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	Renderer    *jsonnet.Renderer
	RenderCache map[types.NamespacedName]RenderCacheEntry
}

func (r *DashboardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("dashboard", req.NamespacedName)

	var dashboard grafanaV1.Dashboard
	if err := r.Get(ctx, req.NamespacedName, &dashboard); err != nil {
		if apierrors.IsNotFound(err) {
			delete(r.RenderCache, req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	grafanaURLs, err := r.resolveGrafanaURLs(ctx, &dashboard)
	if err != nil {
		meta.SetStatusCondition(&dashboard.Status.Conditions, metaV1.Condition{
			Type:    "Ready",
			Status:  metaV1.ConditionFalse,
			Reason:  "EndpointResolveFailed",
			Message: err.Error(),
		})
		if statusErr := r.Status().Update(ctx, &dashboard); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	grafanaHost := fmt.Sprintf("%s.%s.svc.cluster.local", dashboard.Spec.GrafanaServiceRef.Name, dashboard.Spec.GrafanaServiceRef.Namespace)

	if !dashboard.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&dashboard, finalizerName) {
			if dashboard.Status.UID != "" {
				for _, grafanaURL := range grafanaURLs {
					grafanaClient := grafana.NewClient(grafanaURL, grafanaHost)
					if err := grafanaClient.DeleteDashboard(ctx, dashboard.Status.UID); err != nil {
						logger.Error(err, "failed to delete dashboard from Grafana", "grafanaURL", grafanaURL)
					}
				}
				r.Recorder.Event(&dashboard, coreV1.EventTypeNormal, "Deleted", "Dashboard deleted from Grafana")
			}

			controllerutil.RemoveFinalizer(&dashboard, finalizerName)
			if err := r.Update(ctx, &dashboard); err != nil {
				return ctrl.Result{}, err
			}
		}
		delete(r.RenderCache, req.NamespacedName)
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
		meta.SetStatusCondition(&dashboard.Status.Conditions, metaV1.Condition{
			Type:    "Ready",
			Status:  metaV1.ConditionFalse,
			Reason:  "ResolveFailed",
			Message: err.Error(),
		})
		if statusErr := r.Status().Update(ctx, &dashboard); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	if dashboardJSON == nil {
		meta.SetStatusCondition(&dashboard.Status.Conditions, metaV1.Condition{
			Type:    "Ready",
			Status:  metaV1.ConditionFalse,
			Reason:  "InvalidSpec",
			Message: "one of jsonnet or configMapRef must be specified",
		})
		if statusErr := r.Status().Update(ctx, &dashboard); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, nil
	}

	var dashboardResponse *grafana.DashboardResponse
	for _, grafanaURL := range grafanaURLs {
		response, err := r.syncToEndpoint(ctx, grafanaURL, grafanaHost, &dashboard, dashboardJSON)
		if err != nil {
			meta.SetStatusCondition(&dashboard.Status.Conditions, metaV1.Condition{
				Type:    "Ready",
				Status:  metaV1.ConditionFalse,
				Reason:  "SyncFailed",
				Message: err.Error(),
			})
			if statusErr := r.Status().Update(ctx, &dashboard); statusErr != nil {
				logger.Error(statusErr, "failed to update status")
			}
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		if response != nil {
			dashboardResponse = response
		}
	}

	now := metaV1.Now()
	if dashboardResponse != nil {
		dashboard.Status.UID = dashboardResponse.UID
		dashboard.Status.URL = dashboardResponse.URL
		dashboard.Status.Version = dashboardResponse.Version
	}
	dashboard.Status.LastSyncedAt = &now
	meta.SetStatusCondition(&dashboard.Status.Conditions, metaV1.Condition{
		Type:    "Ready",
		Status:  metaV1.ConditionTrue,
		Reason:  "Synced",
		Message: "Dashboard synced to Grafana",
	})

	if err := r.Status().Update(ctx, &dashboard); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(&dashboard, coreV1.EventTypeNormal, "Synced", "Dashboard synced to Grafana")

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *DashboardReconciler) syncToEndpoint(ctx context.Context, grafanaURL string, grafanaHost string, dashboard *grafanaV1.Dashboard, dashboardJSON []byte) (*grafana.DashboardResponse, error) {
	grafanaClient := grafana.NewClient(grafanaURL, grafanaHost)

	folderUID, err := grafanaClient.EnsureFolder(ctx, dashboard.Spec.Folder)
	if err != nil {
		return nil, xerrors.Errorf("failed to ensure folder on %s: %w", grafanaURL, err)
	}

	if dashboard.Status.UID != "" {
		currentDashboard, err := grafanaClient.GetDashboard(ctx, dashboard.Status.UID)
		if err != nil {
			r.Log.Error(err, "failed to get current dashboard from Grafana", "grafanaURL", grafanaURL)
		} else if currentDashboard != nil && dashboardsEqual(dashboardJSON, currentDashboard) {
			r.Log.V(1).Info("dashboard unchanged, skipping sync", "grafanaURL", grafanaURL)
			return nil, nil
		}
	}

	dashboardResponse, err := grafanaClient.UpsertDashboard(ctx, dashboardJSON, folderUID)
	if err != nil {
		return nil, xerrors.Errorf("failed to upsert dashboard on %s: %w", grafanaURL, err)
	}

	if dashboard.Spec.HomeDashboard {
		prefs, err := grafanaClient.GetOrgPreferences(ctx)
		if err != nil {
			return nil, xerrors.Errorf("failed to get org preferences on %s: %w", grafanaURL, err)
		}

		if prefs.HomeDashboardUID != "" && prefs.HomeDashboardUID != dashboardResponse.UID {
			return nil, xerrors.Errorf("another dashboard is already set as home on %s: %s", grafanaURL, prefs.HomeDashboardUID)
		}

		if prefs.HomeDashboardUID != dashboardResponse.UID {
			if err := grafanaClient.SetHomeDashboard(ctx, dashboardResponse.UID); err != nil {
				return nil, xerrors.Errorf("failed to set home dashboard on %s: %w", grafanaURL, err)
			}
			r.Recorder.Event(dashboard, coreV1.EventTypeNormal, "HomeDashboardSet", "Dashboard set as home")
		}
	}

	return dashboardResponse, nil
}

func (r *DashboardReconciler) resolveGrafanaURLs(ctx context.Context, dashboard *grafanaV1.Dashboard) ([]string, error) {
	ref := dashboard.Spec.GrafanaServiceRef

	var endpointSlices discoveryV1.EndpointSliceList
	if err := r.List(ctx, &endpointSlices, &client.ListOptions{
		Namespace: ref.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			discoveryV1.LabelServiceName: ref.Name,
		}),
	}); err != nil {
		return nil, xerrors.Errorf("failed to list endpointslices for service %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	var grafanaURLs []string
	for _, es := range endpointSlices.Items {
		port := grafanaPortFallback
		for _, p := range es.Ports {
			if p.Port != nil {
				port = *p.Port
				if p.Name != nil && *p.Name == grafanaPortName {
					break
				}
			}
		}

		for _, endpoint := range es.Endpoints {
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}
			for _, addr := range endpoint.Addresses {
				grafanaURLs = append(grafanaURLs, fmt.Sprintf("http://%s:%d", addr, port))
			}
		}
	}

	if len(grafanaURLs) == 0 {
		return nil, xerrors.Errorf("no ready endpoints found for service %s/%s", ref.Namespace, ref.Name)
	}

	return grafanaURLs, nil
}

func (r *DashboardReconciler) resolveDashboardContent(ctx context.Context, dashboard *grafanaV1.Dashboard) ([]byte, error) {
	key := types.NamespacedName{Name: dashboard.Name, Namespace: dashboard.Namespace}
	var content string
	var configMapResourceVersion string

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
		configMapResourceVersion = configMap.ResourceVersion
	default:
		return nil, nil
	}

	if cached, ok := r.RenderCache[key]; ok {
		if cached.Generation == dashboard.Generation && cached.ConfigMapResourceVersion == configMapResourceVersion {
			return cached.RenderedJSON, nil
		}
	}

	rendered, err := r.Renderer.Render(content)
	if err != nil {
		return nil, err
	}

	r.RenderCache[key] = RenderCacheEntry{
		Generation:               dashboard.Generation,
		ConfigMapResourceVersion: configMapResourceVersion,
		RenderedJSON:             rendered,
	}

	return rendered, nil
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
