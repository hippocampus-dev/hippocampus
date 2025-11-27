package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	zapV1 "owasp-zap-controller/api/v1"
	"time"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

type ScanReportsRequest map[string]string

func UpdateScanReports(dynamicClient *dynamic.DynamicClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		group := r.PathValue("group")
		version := r.PathValue("version")
		kind := r.PathValue("kind")
		name := r.PathValue("name")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to read request body: %s", err))
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var request ScanReportsRequest
		if err := json.Unmarshal(body, &request); err != nil {
			slog.Error(fmt.Sprintf("failed to unmarshal request: %s", err))
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		var patchData []byte
		switch kind {
		case "scan":
			status := zapV1.ScanStatus{
				LastScanTime: &metaV1.Time{Time: time.Now()},
				Reports:      request,
			}

			statusPatch := map[string]interface{}{
				"status": status,
			}

			patchData, err = json.Marshal(statusPatch)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to marshal patch data: %s", err))
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
			metaV1.PatchOptions{},
			"status",
		)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to patch status: %s", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		b, err := u.MarshalJSON()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to marshal json: %s", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}
}
