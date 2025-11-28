package routes

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	v1 "snapshot-controller/api/v1"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

type ArtifactsRequest struct {
	BaselineURL          string  `json:"baselineURL"`
	TargetURL            string  `json:"targetURL"`
	BaselineHTMLURL      string  `json:"baselineHTMLURL"`
	TargetHTMLURL        string  `json:"targetHTMLURL"`
	ScreenshotDiffURL    string  `json:"screenshotDiffURL"`
	ScreenshotDiffAmount float64 `json:"screenshotDiffAmount"`
	HTMLDiffURL          string  `json:"htmlDiffURL"`
	HTMLDiffAmount       float64 `json:"htmlDiffAmount"`
}

func UpdateArtifacts(dynamicClient *dynamic.DynamicClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		group := r.PathValue("group")
		version := r.PathValue("version")
		kind := r.PathValue("kind")
		name := r.PathValue("name")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var request ArtifactsRequest
		if err := json.Unmarshal(body, &request); err != nil {
			slog.Error("failed to unmarshal request", "error", err)
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		var patchData []byte
		switch kind {
		case "snapshot":
			status := v1.SnapshotStatus{
				BaselineURL:          request.BaselineURL,
				TargetURL:            request.TargetURL,
				BaselineHTMLURL:      request.BaselineHTMLURL,
				TargetHTMLURL:        request.TargetHTMLURL,
				ScreenshotDiffURL:    request.ScreenshotDiffURL,
				ScreenshotDiffAmount: request.ScreenshotDiffAmount,
				HTMLDiffURL:          request.HTMLDiffURL,
				HTMLDiffAmount:       request.HTMLDiffAmount,
				LastSnapshotTime:     &metav1.Time{Time: time.Now()},
			}

			statusPatch := map[string]interface{}{
				"status": status,
			}

			patchData, err = json.Marshal(statusPatch)
			if err != nil {
				slog.Error("failed to marshal patch data", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		case "scheduledsnapshot":
			status := v1.ScheduledSnapshotStatus{
				BaselineURL:          request.BaselineURL,
				TargetURL:            request.TargetURL,
				BaselineHTMLURL:      request.BaselineHTMLURL,
				TargetHTMLURL:        request.TargetHTMLURL,
				ScreenshotDiffURL:    request.ScreenshotDiffURL,
				ScreenshotDiffAmount: request.ScreenshotDiffAmount,
				HTMLDiffURL:          request.HTMLDiffURL,
				HTMLDiffAmount:       request.HTMLDiffAmount,
				LastSnapshotTime:     &metav1.Time{Time: time.Now()},
			}

			statusPatch := map[string]interface{}{
				"status": status,
			}

			patchData, err = json.Marshal(statusPatch)
			if err != nil {
				slog.Error("failed to marshal patch data", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "Unsupported resource kind", http.StatusBadRequest)
			return
		}

		gvr := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: kind + "s",
		}

		u, err := dynamicClient.Resource(gvr).Namespace(namespace).Patch(
			r.Context(),
			name,
			types.MergePatchType,
			patchData,
			metav1.PatchOptions{},
			"status",
		)
		if err != nil {
			slog.Error("failed to patch status", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		b, err := u.MarshalJSON()
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
