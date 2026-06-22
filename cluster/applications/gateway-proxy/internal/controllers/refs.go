package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func isGatewayRef(ref gatewayv1.ParentReference) bool {
	if ref.Group != nil && *ref.Group != gatewayv1.GroupName {
		return false
	}
	if ref.Kind != nil && *ref.Kind != "Gateway" {
		return false
	}
	return true
}

func isListenerSetRef(ref gatewayv1.ParentReference) bool {
	if ref.Group == nil || *ref.Group != gatewayv1.GroupName {
		return false
	}
	if ref.Kind == nil || *ref.Kind != "ListenerSet" {
		return false
	}
	return true
}

func gatewayIndexFunc(parentRefs []gatewayv1.ParentReference, routeNamespace string) []string {
	var gateways []string
	for _, ref := range parentRefs {
		if !isGatewayRef(ref) {
			continue
		}
		namespace := routeNamespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}
		gateways = append(gateways, types.NamespacedName{
			Namespace: namespace,
			Name:      string(ref.Name),
		}.String())
	}
	return gateways
}

func listenerSetIndexFunc(parentRefs []gatewayv1.ParentReference, routeNamespace string) []string {
	var listenerSets []string
	for _, ref := range parentRefs {
		if !isListenerSetRef(ref) {
			continue
		}
		namespace := routeNamespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}
		listenerSets = append(listenerSets, types.NamespacedName{
			Namespace: namespace,
			Name:      string(ref.Name),
		}.String())
	}
	return listenerSets
}

func routeParentGateways(ctx context.Context, reader client.Reader, parentRefs []gatewayv1.ParentReference, routeNamespace string) []ctrl.Request {
	var requests []ctrl.Request
	for _, ref := range parentRefs {
		if isGatewayRef(ref) {
			namespace := routeNamespace
			if ref.Namespace != nil {
				namespace = string(*ref.Namespace)
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      string(ref.Name),
					Namespace: namespace,
				},
			})
			continue
		}
		if !isListenerSetRef(ref) {
			continue
		}

		namespace := routeNamespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}

		listenerSet := &gatewayv1.ListenerSet{}
		if err := reader.Get(ctx, types.NamespacedName{Name: string(ref.Name), Namespace: namespace}, listenerSet); err != nil {
			continue
		}
		gatewayNamespace := listenerSet.Namespace
		if listenerSet.Spec.ParentRef.Namespace != nil {
			gatewayNamespace = string(*listenerSet.Spec.ParentRef.Namespace)
		}
		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      string(listenerSet.Spec.ParentRef.Name),
				Namespace: gatewayNamespace,
			},
		})
	}
	return requests
}

type backendReference struct {
	Name      string
	Namespace string
	Port      gatewayv1.PortNumber
}

func normalizeBackendReference(ref gatewayv1.BackendRef, routeNamespace string) (backendReference, string, bool) {
	if ref.Group != nil && *ref.Group != "" {
		return backendReference{}, string(gatewayv1.RouteReasonInvalidKind), false
	}
	if ref.Kind != nil && *ref.Kind != "Service" {
		return backendReference{}, string(gatewayv1.RouteReasonInvalidKind), false
	}
	if ref.Port == nil {
		return backendReference{}, string(gatewayv1.RouteReasonUnsupportedValue), false
	}

	namespace := routeNamespace
	if ref.Namespace != nil {
		namespace = string(*ref.Namespace)
	}

	return backendReference{
		Name:      string(ref.Name),
		Namespace: namespace,
		Port:      *ref.Port,
	}, "", true
}

func serviceHasPort(service *corev1.Service, port gatewayv1.PortNumber) bool {
	for _, servicePort := range service.Spec.Ports {
		if servicePort.Port == port {
			return true
		}
	}
	return false
}

func usedListenerPortMapKey(port int32, protocol gatewayv1.ProtocolType) string {
	return fmt.Sprintf("%d/%s", port, protocol)
}
