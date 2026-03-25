package handler

import (
	"context"
	"fmt"
	"gateway-proxy/internal/controllers"
	"net/http"

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

	usedPorts, err := h.collectUsedPorts(ctx, gateway.Name, gateway.Namespace, "", "")
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	selfPorts := make(map[portProtocolKey]string)
	for _, listener := range gateway.Spec.Listeners {
		key := portProtocolKey{Port: int32(listener.Port), Protocol: listener.Protocol}
		if existingName, ok := selfPorts[key]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is used by multiple listeners: %s and %s", listener.Port, listener.Protocol, existingName, string(listener.Name)))
		}
		if owner, ok := usedPorts[key]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is already used by %s", listener.Port, listener.Protocol, owner))
		}
		selfPorts[key] = string(listener.Name)
	}

	return admission.Allowed("")
}

func (h *GatewayHandler) handleListenerSet(ctx context.Context, req admission.Request) admission.Response {
	ls := &gatewayv1.ListenerSet{}
	if err := h.Decoder.DecodeRaw(req.Object, ls); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	ref := ls.Spec.ParentRef
	gatewayNamespace := ls.Namespace
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

	usedPorts, err := h.collectUsedPorts(ctx, "", "", ls.Name, ls.Namespace)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	selfPorts := make(map[portProtocolKey]string)
	for _, entry := range ls.Spec.Listeners {
		key := portProtocolKey{Port: int32(entry.Port), Protocol: entry.Protocol}
		if existingName, ok := selfPorts[key]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is used by multiple listeners: %s and %s", entry.Port, entry.Protocol, existingName, string(entry.Name)))
		}
		if owner, ok := usedPorts[key]; ok {
			return admission.Denied(fmt.Sprintf("port %d/%s is already used by %s", entry.Port, entry.Protocol, owner))
		}
		selfPorts[key] = string(entry.Name)
	}

	return admission.Allowed("")
}

func (h *GatewayHandler) collectUsedPorts(ctx context.Context, excludeGatewayName string, excludeGatewayNamespace string, excludeListenerSetName string, excludeListenerSetNamespace string) (map[portProtocolKey]string, error) {
	usedPorts := make(map[portProtocolKey]string)

	classNames, err := h.Resolver.OwnedGatewayClassNames(ctx)
	if err != nil {
		return nil, err
	}

	var gatewayList gatewayv1.GatewayList
	for _, className := range classNames {
		var list gatewayv1.GatewayList
		if err := h.Client.List(ctx, &list, client.MatchingFields{controllers.GatewayClassNameIndex: className}); err != nil {
			return nil, err
		}
		gatewayList.Items = append(gatewayList.Items, list.Items...)
	}

	for _, gw := range gatewayList.Items {
		if gw.Name == excludeGatewayName && gw.Namespace == excludeGatewayNamespace {
			continue
		}
		for _, listener := range gw.Spec.Listeners {
			key := portProtocolKey{Port: int32(listener.Port), Protocol: listener.Protocol}
			usedPorts[key] = fmt.Sprintf("Gateway %s/%s", gw.Namespace, gw.Name)
		}

		var listenerSets gatewayv1.ListenerSetList
		gatewayKey := fmt.Sprintf("%s/%s", gw.Namespace, gw.Name)
		if err := h.Client.List(ctx, &listenerSets, client.MatchingFields{controllers.ListenerSetGatewayIndex: gatewayKey}); err != nil {
			return nil, err
		}
		for _, ls := range listenerSets.Items {
			if ls.Name == excludeListenerSetName && ls.Namespace == excludeListenerSetNamespace {
				continue
			}
			if !h.Resolver.IsListenerSetAllowed(ctx, &gw, &ls) {
				continue
			}
			for _, entry := range ls.Spec.Listeners {
				key := portProtocolKey{Port: int32(entry.Port), Protocol: entry.Protocol}
				usedPorts[key] = fmt.Sprintf("ListenerSet %s/%s", ls.Namespace, ls.Name)
			}
		}
	}

	return usedPorts, nil
}
