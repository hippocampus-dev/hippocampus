package controllers

import (
	"context"
	"fmt"
	"sort"

	"gateway-proxy/internal/proxy"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type RouteResolver struct {
	Reader         client.Reader
	ControllerName gatewayv1.GatewayController
}

type portKey struct {
	Port     int32
	Protocol proxy.Protocol
}

type resolvedRoutes struct {
	gatewayTCPRoutes     map[string][]gatewayv1alpha2.TCPRoute
	gatewayUDPRoutes     map[string][]gatewayv1alpha2.UDPRoute
	listenerSetTCPRoutes map[string][]gatewayv1alpha2.TCPRoute
	listenerSetUDPRoutes map[string][]gatewayv1alpha2.UDPRoute
}

func (r *RouteResolver) resolveAllRoutes(ctx context.Context) ([]proxy.Route, map[portKey]bool, *resolvedRoutes, error) {
	gateways, err := r.listOwnedGateways(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	resolved := &resolvedRoutes{
		gatewayTCPRoutes:     make(map[string][]gatewayv1alpha2.TCPRoute),
		gatewayUDPRoutes:     make(map[string][]gatewayv1alpha2.UDPRoute),
		listenerSetTCPRoutes: make(map[string][]gatewayv1alpha2.TCPRoute),
		listenerSetUDPRoutes: make(map[string][]gatewayv1alpha2.UDPRoute),
	}
	var allRoutes []proxy.Route

	for _, gateway := range gateways {
		gatewayRoutes, err := r.collectGatewayRoutes(ctx, &gateway, resolved)
		if err != nil {
			return nil, nil, nil, err
		}
		allRoutes = append(allRoutes, gatewayRoutes...)

		listenerSetRoutes, err := r.collectListenerSetRoutes(ctx, &gateway, resolved)
		if err != nil {
			return nil, nil, nil, err
		}
		allRoutes = append(allRoutes, listenerSetRoutes...)
	}

	return filterConflictedRoutes(allRoutes, resolved)
}

func (r *RouteResolver) collectGatewayRoutes(ctx context.Context, gateway *gatewayv1.Gateway, resolved *resolvedRoutes) ([]proxy.Route, error) {
	gatewayKey := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}.String()

	var tcpRouteList gatewayv1alpha2.TCPRouteList
	if err := r.Reader.List(ctx, &tcpRouteList, client.MatchingFields{TCPRouteGatewayIndex: gatewayKey}); err != nil {
		return nil, err
	}
	resolved.gatewayTCPRoutes[gatewayKey] = tcpRouteList.Items

	var udpRouteList gatewayv1alpha2.UDPRouteList
	if err := r.Reader.List(ctx, &udpRouteList, client.MatchingFields{UDPRouteGatewayIndex: gatewayKey}); err != nil {
		return nil, err
	}
	resolved.gatewayUDPRoutes[gatewayKey] = udpRouteList.Items

	var routes []proxy.Route
	for _, tcpRoute := range tcpRouteList.Items {
		for _, rule := range tcpRoute.Spec.Rules {
			backends := r.resolveBackendRefs(ctx, rule.BackendRefs, tcpRoute.Namespace, "TCPRoute")
			if len(backends) == 0 {
				continue
			}
			for _, parentRef := range tcpRoute.Spec.ParentRefs {
				if !isGatewayRef(parentRef) || string(parentRef.Name) != gateway.Name ||
					(parentRef.Namespace != nil && string(*parentRef.Namespace) != gateway.Namespace) {
					continue
				}
				for _, port := range allowedListenerPorts(ctx, r.Reader, gateway, parentRef.SectionName, tcpRoute.Namespace) {
					routes = append(routes, proxy.Route{
						Port:     port,
						Protocol: proxy.ProtocolTCP,
						Backends: backends,
					})
				}
			}
		}
	}

	for _, udpRoute := range udpRouteList.Items {
		for _, rule := range udpRoute.Spec.Rules {
			backends := r.resolveBackendRefs(ctx, rule.BackendRefs, udpRoute.Namespace, "UDPRoute")
			if len(backends) == 0 {
				continue
			}
			for _, parentRef := range udpRoute.Spec.ParentRefs {
				if !isGatewayRef(parentRef) || string(parentRef.Name) != gateway.Name ||
					(parentRef.Namespace != nil && string(*parentRef.Namespace) != gateway.Namespace) {
					continue
				}
				for _, port := range allowedListenerPorts(ctx, r.Reader, gateway, parentRef.SectionName, udpRoute.Namespace) {
					routes = append(routes, proxy.Route{
						Port:     port,
						Protocol: proxy.ProtocolUDP,
						Backends: backends,
					})
				}
			}
		}
	}

	return routes, nil
}

func (r *RouteResolver) collectListenerSetRoutes(ctx context.Context, gateway *gatewayv1.Gateway, resolved *resolvedRoutes) ([]proxy.Route, error) {
	attachedListenerSets, err := r.getAttachedListenerSets(ctx, gateway)
	if err != nil {
		return nil, err
	}

	var routes []proxy.Route
	for _, listenerSet := range attachedListenerSets {
		listenerSetKey := types.NamespacedName{Name: listenerSet.Name, Namespace: listenerSet.Namespace}.String()

		var tcpRouteList gatewayv1alpha2.TCPRouteList
		if err := r.Reader.List(ctx, &tcpRouteList, client.MatchingFields{TCPRouteListenerSetIndex: listenerSetKey}); err != nil {
			return nil, err
		}
		resolved.listenerSetTCPRoutes[listenerSetKey] = tcpRouteList.Items

		var udpRouteList gatewayv1alpha2.UDPRouteList
		if err := r.Reader.List(ctx, &udpRouteList, client.MatchingFields{UDPRouteListenerSetIndex: listenerSetKey}); err != nil {
			return nil, err
		}
		resolved.listenerSetUDPRoutes[listenerSetKey] = udpRouteList.Items

		for _, tcpRoute := range tcpRouteList.Items {
			for _, rule := range tcpRoute.Spec.Rules {
				backends := r.resolveBackendRefs(ctx, rule.BackendRefs, tcpRoute.Namespace, "TCPRoute")
				if len(backends) == 0 {
					continue
				}
				for _, parentRef := range tcpRoute.Spec.ParentRefs {
					if !isListenerSetRef(parentRef) || string(parentRef.Name) != listenerSet.Name ||
						(parentRef.Namespace != nil && string(*parentRef.Namespace) != listenerSet.Namespace) {
						continue
					}
					for _, port := range allowedListenerSetListenerPorts(ctx, r.Reader, &listenerSet, parentRef.SectionName, tcpRoute.Namespace, gateway.Namespace) {
						routes = append(routes, proxy.Route{
							Port:     port,
							Protocol: proxy.ProtocolTCP,
							Backends: backends,
						})
					}
				}
			}
		}

		for _, udpRoute := range udpRouteList.Items {
			for _, rule := range udpRoute.Spec.Rules {
				backends := r.resolveBackendRefs(ctx, rule.BackendRefs, udpRoute.Namespace, "UDPRoute")
				if len(backends) == 0 {
					continue
				}
				for _, parentRef := range udpRoute.Spec.ParentRefs {
					if !isListenerSetRef(parentRef) || string(parentRef.Name) != listenerSet.Name ||
						(parentRef.Namespace != nil && string(*parentRef.Namespace) != listenerSet.Namespace) {
						continue
					}
					for _, port := range allowedListenerSetListenerPorts(ctx, r.Reader, &listenerSet, parentRef.SectionName, udpRoute.Namespace, gateway.Namespace) {
						routes = append(routes, proxy.Route{
							Port:     port,
							Protocol: proxy.ProtocolUDP,
							Backends: backends,
						})
					}
				}
			}
		}
	}

	return routes, nil
}

func filterConflictedRoutes(allRoutes []proxy.Route, resolved *resolvedRoutes) ([]proxy.Route, map[portKey]bool, *resolvedRoutes, error) {
	keyCount := make(map[portKey]int)
	for _, route := range allRoutes {
		keyCount[portKey{Port: route.Port, Protocol: route.Protocol}]++
	}

	conflictedPorts := make(map[portKey]bool)
	for key, count := range keyCount {
		if count > 1 {
			conflictedPorts[key] = true
		}
	}

	if len(conflictedPorts) == 0 {
		return allRoutes, nil, resolved, nil
	}

	var routes []proxy.Route
	for _, route := range allRoutes {
		if !conflictedPorts[portKey{Port: route.Port, Protocol: route.Protocol}] {
			routes = append(routes, route)
		}
	}
	return routes, conflictedPorts, resolved, nil
}

func (r *RouteResolver) listOwnedGateways(ctx context.Context) ([]gatewayv1.Gateway, error) {
	classNames, err := r.ownedGatewayClassNames(ctx)
	if err != nil {
		return nil, err
	}

	var gatewayList gatewayv1.GatewayList
	for _, className := range classNames {
		var list gatewayv1.GatewayList
		if err := r.Reader.List(ctx, &list, client.MatchingFields{GatewayClassNameIndex: className}); err != nil {
			return nil, err
		}
		gatewayList.Items = append(gatewayList.Items, list.Items...)
	}

	return gatewayList.Items, nil
}

func (r *RouteResolver) CollectUsedListenerPorts(ctx context.Context, excludeGateway *types.NamespacedName, excludeListenerSet *types.NamespacedName) (map[string]string, error) {
	usedPorts := make(map[string]string)

	gateways, err := r.listOwnedGateways(ctx)
	if err != nil {
		return nil, err
	}

	for _, gateway := range gateways {
		if excludeGateway == nil || gateway.Name != excludeGateway.Name || gateway.Namespace != excludeGateway.Namespace {
			for _, listener := range gateway.Spec.Listeners {
				key := usedListenerPortMapKey(listener.Port, listener.Protocol)
				usedPorts[key] = fmt.Sprintf("Gateway %s/%s", gateway.Namespace, gateway.Name)
			}
		}

		listenerSets, err := r.getAttachedListenerSets(ctx, &gateway)
		if err != nil {
			return nil, err
		}
		for _, listenerSet := range listenerSets {
			if excludeListenerSet != nil && listenerSet.Name == excludeListenerSet.Name && listenerSet.Namespace == excludeListenerSet.Namespace {
				continue
			}
			for _, entry := range listenerSet.Spec.Listeners {
				key := usedListenerPortMapKey(int32(entry.Port), entry.Protocol)
				usedPorts[key] = fmt.Sprintf("ListenerSet %s/%s", listenerSet.Namespace, listenerSet.Name)
			}
		}
	}

	return usedPorts, nil
}

func (r *RouteResolver) ownedGatewayClassNames(ctx context.Context) ([]string, error) {
	var gatewayClassList gatewayv1.GatewayClassList
	if err := r.Reader.List(ctx, &gatewayClassList); err != nil {
		return nil, err
	}
	var names []string
	for _, gatewayClass := range gatewayClassList.Items {
		if gatewayClass.Spec.ControllerName == r.ControllerName {
			names = append(names, gatewayClass.Name)
		}
	}
	return names, nil
}

func (r *RouteResolver) getAttachedListenerSets(ctx context.Context, gateway *gatewayv1.Gateway) ([]gatewayv1.ListenerSet, error) {
	gatewayKey := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}.String()

	var allListenerSets gatewayv1.ListenerSetList
	if err := r.Reader.List(ctx, &allListenerSets, client.MatchingFields{ListenerSetGatewayIndex: gatewayKey}); err != nil {
		return nil, err
	}

	var attached []gatewayv1.ListenerSet
	for _, listenerSet := range allListenerSets.Items {
		if r.isListenerSetAllowed(ctx, gateway, &listenerSet) {
			attached = append(attached, listenerSet)
		}
	}

	sort.Slice(attached, func(i int, j int) bool {
		iTime := attached[i].CreationTimestamp.Time
		jTime := attached[j].CreationTimestamp.Time
		if !iTime.Equal(jTime) {
			return iTime.Before(jTime)
		}
		iKey := fmt.Sprintf("%s/%s", attached[i].Namespace, attached[i].Name)
		jKey := fmt.Sprintf("%s/%s", attached[j].Namespace, attached[j].Name)
		return iKey < jKey
	})

	return attached, nil
}

func (r *RouteResolver) isListenerSetAllowed(ctx context.Context, gateway *gatewayv1.Gateway, listenerSet *gatewayv1.ListenerSet) bool {
	if gateway.Spec.AllowedListeners == nil {
		return false
	}
	namespaces := gateway.Spec.AllowedListeners.Namespaces
	if namespaces == nil {
		return false
	}
	if namespaces.From == nil {
		return false
	}

	switch *namespaces.From {
	case gatewayv1.NamespacesFromAll:
		return true
	case gatewayv1.NamespacesFromSame:
		return listenerSet.Namespace == gateway.Namespace
	case gatewayv1.NamespacesFromSelector:
		if namespaces.Selector == nil {
			return false
		}
		selector, err := metav1.LabelSelectorAsSelector(namespaces.Selector)
		if err != nil {
			return false
		}
		namespace := &corev1.Namespace{}
		if err := r.Reader.Get(ctx, types.NamespacedName{Name: listenerSet.Namespace}, namespace); err != nil {
			return false
		}
		return selector.Matches(labels.Set(namespace.Labels))
	case gatewayv1.NamespacesFromNone:
		return false
	default:
		return false
	}
}

func (r *RouteResolver) IsOwnedGateway(ctx context.Context, gateway *gatewayv1.Gateway) (bool, error) {
	gatewayClass := &gatewayv1.GatewayClass{}
	if err := r.Reader.Get(ctx, types.NamespacedName{Name: string(gateway.Spec.GatewayClassName)}, gatewayClass); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return gatewayClass.Spec.ControllerName == r.ControllerName, nil
}

func (r *RouteResolver) backendReferenceFromRef(ctx context.Context, ref gatewayv1.BackendRef, routeNamespace string, routeKind string) (backendReference, string, bool) {
	reference, failureReason, ok := normalizeBackendReference(ref, routeNamespace)
	if !ok {
		return backendReference{}, failureReason, false
	}

	if reference.Namespace != routeNamespace && !r.isReferenceGranted(ctx, routeNamespace, routeKind, reference.Namespace, reference.Name) {
		return backendReference{}, string(gatewayv1.RouteReasonRefNotPermitted), false
	}

	return reference, "", true
}

func (r *RouteResolver) resolveBackendRefs(ctx context.Context, backendRefs []gatewayv1.BackendRef, routeNamespace string, routeKind string) []proxy.Backend {
	var backends []proxy.Backend
	for _, ref := range backendRefs {
		reference, _, ok := r.backendReferenceFromRef(ctx, ref, routeNamespace, routeKind)
		if !ok {
			continue
		}

		backends = append(backends, proxy.Backend{
			Address: fmt.Sprintf("%s.%s.svc.cluster.local", reference.Name, reference.Namespace),
			Port:    reference.Port,
		})
	}
	return backends
}

func (r *RouteResolver) isReferenceGranted(ctx context.Context, fromNamespace string, fromKind string, toNamespace string, toName string) bool {
	var referenceGrants gatewayv1.ReferenceGrantList
	if err := r.Reader.List(ctx, &referenceGrants, client.InNamespace(toNamespace)); err != nil {
		return false
	}
	for _, grant := range referenceGrants.Items {
		fromMatch := false
		for _, from := range grant.Spec.From {
			if string(from.Namespace) == fromNamespace &&
				string(from.Group) == gatewayv1.GroupName &&
				string(from.Kind) == fromKind {
				fromMatch = true
				break
			}
		}
		if !fromMatch {
			continue
		}
		for _, to := range grant.Spec.To {
			if string(to.Group) == "" && string(to.Kind) == "Service" {
				if to.Name == nil || string(*to.Name) == toName {
					return true
				}
			}
		}
	}
	return false
}
