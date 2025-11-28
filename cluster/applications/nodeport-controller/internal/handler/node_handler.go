package handler

import (
	"context"
	"fmt"
	"net/http"
	os "os"
	"strings"

	admissionV1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func apiGroup() string {
	defaultGroup := "nodeport-controller.kaidotio.github.io"
	if v, ok := os.LookupEnv("VARIANT"); ok {
		return fmt.Sprintf("%s.%s", v, defaultGroup)
	}
	return defaultGroup
}

type NodeHandler struct {
	Client  client.Client
	Decoder *admission.Decoder
}

func (h *NodeHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	node := &v1.Node{}
	if err := h.Decoder.DecodeRaw(req.Object, node); err != nil {
		handlerLogger.Error(err, "unable to decode object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionV1.Create:
		var serviceList v1.ServiceList
		if err := h.Client.List(ctx, &serviceList, &client.ListOptions{
			FieldSelector: fields.SelectorFromSet(fields.Set{"spec.type": "NodePort"}),
		}); err != nil {
			handlerLogger.Error(err, "unable to list services")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		for _, service := range serviceList.Items {
			annotation := service.Annotations[fmt.Sprintf("%s/nodes", apiGroup())]
			if annotation == "" {
				continue
			}

			nodes := strings.Split(annotation, ",")
			var m map[string]bool
			for _, n := range nodes {
				m[n] = true
			}

			for _, address := range node.Status.Addresses {
				if address.Type != v1.NodeInternalIP {
					continue
				}

				if !m[address.Address] {
					m[address.Address] = true
				}
			}

			nodes = []string{}
			for n := range m {
				nodes = append(nodes, n)
			}

			service.Annotations[fmt.Sprintf("%s/nodes", apiGroup())] = strings.Join(nodes, ",")
			if err := h.Client.Update(ctx, &service); err != nil {
				handlerLogger.Error(err, "unable to update service")
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}
	case admissionV1.Delete:
		var serviceList v1.ServiceList
		if err := h.Client.List(ctx, &serviceList, &client.ListOptions{
			FieldSelector: fields.SelectorFromSet(fields.Set{"spec.type": "NodePort"}),
		}); err != nil {
			handlerLogger.Error(err, "unable to list services")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		for _, service := range serviceList.Items {
			annotation := service.Annotations[fmt.Sprintf("%s/nodes", apiGroup())]
			if annotation == "" {
				continue
			}

			nodes := strings.Split(annotation, ",")
			m := make(map[string]bool)
			for _, n := range nodes {
				m[n] = true
			}

			for _, address := range node.Status.Addresses {
				if address.Type != v1.NodeInternalIP {
					continue
				}

				if m[address.Address] {
					delete(m, address.Address)
				}
			}

			nodes = []string{}
			for n := range m {
				nodes = append(nodes, n)
			}

			service.Annotations[fmt.Sprintf("%s/nodes", apiGroup())] = strings.Join(nodes, ",")
			if err := h.Client.Update(ctx, &service); err != nil {
				handlerLogger.Error(err, "unable to update service")
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}
	}

	return admission.Allowed("")
}
