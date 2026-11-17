package controllers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func allowedListenerPorts(ctx context.Context, reader client.Reader, gateway *gatewayv1.Gateway, sectionName *gatewayv1.SectionName, routeNamespace string) []int32 {
	var ports []int32
	for _, listener := range gateway.Spec.Listeners {
		if sectionName != nil && listener.Name != *sectionName {
			continue
		}
		if isRouteAllowedByListener(ctx, reader, listener.AllowedRoutes, routeNamespace, gateway.Namespace) {
			ports = append(ports, listener.Port)
		}
	}
	return ports
}

func allowedListenerSetListenerPorts(ctx context.Context, reader client.Reader, listenerSet *gatewayv1.ListenerSet, sectionName *gatewayv1.SectionName, routeNamespace string, gatewayNamespace string) []int32 {
	var ports []int32
	for _, entry := range listenerSet.Spec.Listeners {
		if sectionName != nil && entry.Name != *sectionName {
			continue
		}
		if isRouteAllowedByListener(ctx, reader, entry.AllowedRoutes, routeNamespace, gatewayNamespace) {
			ports = append(ports, entry.Port)
		}
	}
	return ports
}

func isRouteAllowedByListener(ctx context.Context, reader client.Reader, allowedRoutes *gatewayv1.AllowedRoutes, routeNamespace string, ownerNamespace string) bool {
	if allowedRoutes == nil || allowedRoutes.Namespaces == nil || allowedRoutes.Namespaces.From == nil {
		return routeNamespace == ownerNamespace
	}
	switch *allowedRoutes.Namespaces.From {
	case gatewayv1.NamespacesFromAll:
		return true
	case gatewayv1.NamespacesFromSame:
		return routeNamespace == ownerNamespace
	case gatewayv1.NamespacesFromSelector:
		if allowedRoutes.Namespaces.Selector == nil {
			return false
		}
		selector, err := metav1.LabelSelectorAsSelector(allowedRoutes.Namespaces.Selector)
		if err != nil {
			return false
		}
		namespace := &corev1.Namespace{}
		if err := reader.Get(ctx, types.NamespacedName{Name: routeNamespace}, namespace); err != nil {
			return false
		}
		return selector.Matches(labels.Set(namespace.Labels))
	case gatewayv1.NamespacesFromNone:
		return false
	default:
		return false
	}
}
