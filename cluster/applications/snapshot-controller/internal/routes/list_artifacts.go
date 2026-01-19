package routes

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	v1 "snapshot-controller/api/v1"
	"snapshot-controller/internal/storage"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ArtifactsResponse struct {
	Baseline       string  `json:"baseline,omitempty"`
	Target         string  `json:"target,omitempty"`
	BaselineHTML   string  `json:"baselineHtml,omitempty"`
	TargetHTML     string  `json:"targetHtml,omitempty"`
	ScreenshotDiff string  `json:"screenshotDiff,omitempty"`
	HTMLDiff       string  `json:"htmlDiff,omitempty"`
	DiffAmount     float64 `json:"diffAmount,omitempty"`
	HTMLDiffAmount float64 `json:"htmlDiffAmount,omitempty"`
}

func ListArtifacts(dynamicClient *dynamic.DynamicClient, storageClient storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		group := r.PathValue("group")
		version := r.PathValue("version")
		kind := r.PathValue("kind")
		name := r.PathValue("name")

		gvr := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: kind + "s",
		}

		u, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				http.NotFound(w, r)
				return
			}
			slog.Error("failed to get resource", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		var response ArtifactsResponse

		switch kind {
		case "snapshot":
			var snapshot v1.Snapshot
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &snapshot); err != nil {
				slog.Error("failed to convert snapshot", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			response = ArtifactsResponse{
				DiffAmount:     snapshot.Status.ScreenshotDiffAmount,
				HTMLDiffAmount: snapshot.Status.HTMLDiffAmount,
			}

			if snapshot.Status.BaselineURL != "" {
				if data, err := storageClient.Get(r.Context(), snapshot.Status.BaselineURL); err == nil {
					response.Baseline = base64.StdEncoding.EncodeToString(data)
				}
			}
			if snapshot.Status.TargetURL != "" {
				if data, err := storageClient.Get(r.Context(), snapshot.Status.TargetURL); err == nil {
					response.Target = base64.StdEncoding.EncodeToString(data)
				}
			}
			if snapshot.Status.BaselineHTMLURL != "" {
				if data, err := storageClient.Get(r.Context(), snapshot.Status.BaselineHTMLURL); err == nil {
					response.BaselineHTML = base64.StdEncoding.EncodeToString(data)
				}
			}
			if snapshot.Status.TargetHTMLURL != "" {
				if data, err := storageClient.Get(r.Context(), snapshot.Status.TargetHTMLURL); err == nil {
					response.TargetHTML = base64.StdEncoding.EncodeToString(data)
				}
			}
			if snapshot.Status.ScreenshotDiffURL != "" {
				if data, err := storageClient.Get(r.Context(), snapshot.Status.ScreenshotDiffURL); err == nil {
					response.ScreenshotDiff = base64.StdEncoding.EncodeToString(data)
				}
			}
			if snapshot.Status.HTMLDiffURL != "" {
				if data, err := storageClient.Get(r.Context(), snapshot.Status.HTMLDiffURL); err == nil {
					response.HTMLDiff = base64.StdEncoding.EncodeToString(data)
				}
			}
		case "scheduledsnapshot":
			var scheduledSnapshot v1.ScheduledSnapshot
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &scheduledSnapshot); err != nil {
				slog.Error("failed to convert scheduled snapshot", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			response = ArtifactsResponse{
				DiffAmount:     scheduledSnapshot.Status.ScreenshotDiffAmount,
				HTMLDiffAmount: scheduledSnapshot.Status.HTMLDiffAmount,
			}

			if scheduledSnapshot.Status.BaselineURL != "" {
				if data, err := storageClient.Get(r.Context(), scheduledSnapshot.Status.BaselineURL); err == nil {
					response.Baseline = base64.StdEncoding.EncodeToString(data)
				}
			}
			if scheduledSnapshot.Status.TargetURL != "" {
				if data, err := storageClient.Get(r.Context(), scheduledSnapshot.Status.TargetURL); err == nil {
					response.Target = base64.StdEncoding.EncodeToString(data)
				}
			}
			if scheduledSnapshot.Status.BaselineHTMLURL != "" {
				if data, err := storageClient.Get(r.Context(), scheduledSnapshot.Status.BaselineHTMLURL); err == nil {
					response.BaselineHTML = base64.StdEncoding.EncodeToString(data)
				}
			}
			if scheduledSnapshot.Status.TargetHTMLURL != "" {
				if data, err := storageClient.Get(r.Context(), scheduledSnapshot.Status.TargetHTMLURL); err == nil {
					response.TargetHTML = base64.StdEncoding.EncodeToString(data)
				}
			}
			if scheduledSnapshot.Status.ScreenshotDiffURL != "" {
				if data, err := storageClient.Get(r.Context(), scheduledSnapshot.Status.ScreenshotDiffURL); err == nil {
					response.ScreenshotDiff = base64.StdEncoding.EncodeToString(data)
				}
			}
			if scheduledSnapshot.Status.HTMLDiffURL != "" {
				if data, err := storageClient.Get(r.Context(), scheduledSnapshot.Status.HTMLDiffURL); err == nil {
					response.HTMLDiff = base64.StdEncoding.EncodeToString(data)
				}
			}
		default:
			http.Error(w, "unsupported resource kind", http.StatusBadRequest)
			return
		}

		b, err := json.Marshal(response)
		if err != nil {
			slog.Error("failed to marshal json", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}
}
