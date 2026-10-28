package handler

import (
	"context"
	"fmt"
	"gateway-proxy/internal/controllers"
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type portProtocolKey struct {
	Port     int32
	Protocol gatewayv1.ProtocolType
}

type GatewayHandler struct {
	Client   client.Client
	Decoder  admission.Decoder
	Resolver *controllers.RouteResolver
}

func (h *GatewayHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	switch req.Kind.Kind {
	case "Gateway":
		return h.handleGateway(ctx, req)
	case "ListenerSet":
		return h.handleListenerSet(ctx, req)
	default:
		handlerLogger.Info("unknown kind", "kind", req.Kind.Kind)
		return admission.Allowed("")
	}
}

func (h *GatewayHandler) handleGateway(ctx context.Context, req admission.Request) admission.Response {
	gateway := &gatewayv1.Gateway{}
	if err := h.Decoder.DecodeRaw(req.Object, gateway); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	owned, err := h.Resolver.IsOwnedGateway(ctx, gateway)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if !owned {
		return admission.Allowed("")
	}

	usedPorts, err := h.Resolver.CollectUsedListenerPorts(ctx, &types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, nil)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	selfPorts := make(map[portProtocolKey]string)
	for _, listener := range gateway.Spec.Listeners {
		key := portProtocolKey{Port: listener.Port, Protocol: listener.Protocol}
		if existingName, ok := selfPorts[key]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is used by multiple listeners: %s and %s", listener.Port, listener.Protocol, existingName, string(listener.Name)))
		}
		if owner, ok := usedPorts[usedListenerPortMapKey(listener.Port, listener.Protocol)]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is already used by %s", listener.Port, listener.Protocol, owner))
		}
		selfPorts[key] = string(listener.Name)
	}

	return admission.Allowed("")
}

func (h *GatewayHandler) handleListenerSet(ctx context.Context, req admission.Request) admission.Response {
	listenerSet := &gatewayv1.ListenerSet{}
	if err := h.Decoder.DecodeRaw(req.Object, listenerSet); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	ref := listenerSet.Spec.ParentRef
	gatewayNamespace := listenerSet.Namespace
	if ref.Namespace != nil {
		gatewayNamespace = string(*ref.Namespace)
	}

	gateway := &gatewayv1.Gateway{}
	if err := h.Client.Get(ctx, client.ObjectKey{Name: string(ref.Name), Namespace: gatewayNamespace}, gateway); err != nil {
		return admission.Denied(fmt.Sprintf("parent Gateway %s/%s not found", gatewayNamespace, ref.Name))
	}

	owned, err := h.Resolver.IsOwnedGateway(ctx, gateway)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if !owned {
		return admission.Allowed("")
	}

	usedPorts, err := h.Resolver.CollectUsedListenerPorts(ctx, nil, &types.NamespacedName{Name: listenerSet.Name, Namespace: listenerSet.Namespace})
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	selfPorts := make(map[portProtocolKey]string)
	for _, entry := range listenerSet.Spec.Listeners {
		key := portProtocolKey{Port: entry.Port, Protocol: entry.Protocol}
		if existingName, ok := selfPorts[key]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is used by multiple listeners: %s and %s", entry.Port, entry.Protocol, existingName, string(entry.Name)))
		}
		if owner, ok := usedPorts[usedListenerPortMapKey(entry.Port, entry.Protocol)]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is already used by %s", entry.Port, entry.Protocol, owner))
		}
		selfPorts[key] = string(entry.Name)
	}

	return admission.Allowed("")
}

func usedListenerPortMapKey(port int32, protocol gatewayv1.ProtocolType) string {
	return fmt.Sprintf("%d/%s", port, protocol)
}
