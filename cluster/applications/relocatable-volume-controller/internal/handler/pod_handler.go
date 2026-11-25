package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func apiGroup() string {
	defaultGroup := "relocatable-volume-controller.kaidotio.github.io"
	if v, ok := os.LookupEnv("VARIANT"); ok {
		return fmt.Sprintf("%s.%s", v, defaultGroup)
	}
	return defaultGroup
}

type PodHandler struct {
	Reader  client.Reader
	Decoder admission.Decoder
}

func (h *PodHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	pod := &coreV1.Pod{}
	if err := h.Decoder.DecodeRaw(req.Object, pod); err != nil {
		handlerLogger.Error(err, "unable to decode object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	value, ok := pod.Annotations[fmt.Sprintf("%s/volumes", apiGroup())]
	if !ok {
		return admission.Allowed("")
	}
	relocatableVolumes := strings.Split(value, ",")

	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil || !slices.Contains(relocatableVolumes, volume.Name) {
			continue
		}

		claim := &coreV1.PersistentVolumeClaim{}
		if err := h.Reader.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: volume.PersistentVolumeClaim.ClaimName}, claim); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			handlerLogger.Error(err, "unable to get persistentvolumeclaim")
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if _, ok := claim.Annotations[fmt.Sprintf("%s/relocating-from", apiGroup())]; !ok {
			continue
		}

		pod.Spec.SchedulingGates = append(pod.Spec.SchedulingGates, coreV1.PodSchedulingGate{Name: fmt.Sprintf("%s/relocating", apiGroup())})
		marshaled, err := json.Marshal(pod)
		if err != nil {
			handlerLogger.Error(err, "unable to marshal pod")
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
	}

	return admission.Allowed("")
}
