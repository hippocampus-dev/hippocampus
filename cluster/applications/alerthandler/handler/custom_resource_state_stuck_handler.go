package handler

import (
	"context"
	"strings"
	"time"

	"golang.org/x/xerrors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

type CustomResourceStateStuckHandler struct {
	client   dynamic.Interface
	cooldown time.Duration
}

func NewCustomResourceStateStuckHandler(client dynamic.Interface, cooldown time.Duration) *CustomResourceStateStuckHandler {
	return &CustomResourceStateStuckHandler{
		client:   client,
		cooldown: cooldown,
	}
}

func (h *CustomResourceStateStuckHandler) Call(request *AlertManagerRequest) error {
	ctx := context.Background()
	for _, alert := range request.Alerts {
		group := alert.Labels["customresource_group"]
		version := alert.Labels["customresource_version"]
		kind := alert.Labels["customresource_kind"]
		namespace := alert.Labels["namespace"]
		name := alert.Labels["name"]
		if group == "" || version == "" || kind == "" || namespace == "" || name == "" {
			return xerrors.New("alert must have customresource_group,customresource_version,customresource_kind,namespace,name labels")
		}

		resource := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: strings.ToLower(kind) + "s",
		}

		cr, err := h.client.Resource(resource).Namespace(namespace).Get(ctx, name, metaV1.GetOptions{})
		if err != nil {
			return xerrors.Errorf("failed to get %s %s/%s: %w", kind, namespace, name, err)
		}

		annotations := cr.GetAnnotations()
		if annotations != nil {
			if lastPatch, ok := annotations["alerthandler.kaidotio.github.io/restartedAt"]; ok {
				if t, err := time.Parse(time.RFC3339, lastPatch); err == nil {
					if time.Since(t) < h.cooldown {
						continue
					}
				}
			}
		}

		patch := []byte(`{"metadata":{"annotations":{"alerthandler.kaidotio.github.io/restartedAt":"` + time.Now().UTC().Format(time.RFC3339) + `"}}}`)
		if _, err := h.client.Resource(resource).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patch, metaV1.PatchOptions{}); err != nil {
			return xerrors.Errorf("failed to patch %s %s/%s: %w", kind, namespace, name, err)
		}
	}
	return nil
}
