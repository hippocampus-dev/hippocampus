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
	classNames, err := r.OwnedGatewayClassNames(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	var gatewayList gatewayv1.GatewayList
	for _, className := range classNames {
		var list gatewayv1.GatewayList
		if err := r.Reader.List(ctx, &list, client.MatchingFields{GatewayClassNameIndex: className}); err != nil {
			return nil, nil, nil, err
		}
		gatewayList.Items = append(gatewayList.Items, list.Items...)
	}

	resolved := &resolvedRoutes{
		gatewayTCPRoutes:     make(map[string][]gatewayv1alpha2.TCPRoute),
		gatewayUDPRoutes:     make(map[string][]gatewayv1alpha2.UDPRoute),
		listenerSetTCPRoutes: make(map[string][]gatewayv1alpha2.TCPRoute),
		listenerSetUDPRoutes: make(map[string][]gatewayv1alpha2.UDPRoute),
	}
	var allRoutes []proxy.Route

	for _, gw := range gatewayList.Items {
		gwRoutes, err := r.collectGatewayRoutes(ctx, &gw, resolved)
		if err != nil {
			return nil, nil, nil, err
		}
		allRoutes = append(allRoutes, gwRoutes...)

		lsRoutes, err := r.collectListenerSetRoutes(ctx, &gw, resolved)
		if err != nil {
			return nil, nil, nil, err
		}
		allRoutes = append(allRoutes, lsRoutes...)
	}

	return filterConflictedRoutes(allRoutes, resolved)
}

func (r *RouteResolver) collectGatewayRoutes(ctx context.Context, gw *gatewayv1.Gateway, resolved *resolvedRoutes) ([]proxy.Route, error) {
	gatewayKey := types.NamespacedName{Name: gw.Name, Namespace: gw.Namespace}.String()

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
				if !isGatewayRef(parentRef) || string(parentRef.Name) != gw.Name ||
					(parentRef.Namespace != nil && string(*parentRef.Namespace) != gw.Namespace) {
					continue
				}
				for _, port := range allowedListenerPorts(ctx, r.Reader, gw, parentRef.SectionName, tcpRoute.Namespace) {
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
				if !isGatewayRef(parentRef) || string(parentRef.Name) != gw.Name ||
					(parentRef.Namespace != nil && string(*parentRef.Namespace) != gw.Namespace) {
					continue
				}
				for _, port := range allowedListenerPorts(ctx, r.Reader, gw, parentRef.SectionName, udpRoute.Namespace) {
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

func (r *RouteResolver) collectListenerSetRoutes(ctx context.Context, gw *gatewayv1.Gateway, resolved *resolvedRoutes) ([]proxy.Route, error) {
	attachedListenerSets, err := r.getAttachedListenerSets(ctx, gw)
	if err != nil {
		return nil, err
	}

	var routes []proxy.Route
	for _, ls := range attachedListenerSets {
		lsKey := types.NamespacedName{Name: ls.Name, Namespace: ls.Namespace}.String()

		var lsTCPRouteList gatewayv1alpha2.TCPRouteList
		if err := r.Reader.List(ctx, &lsTCPRouteList, client.MatchingFields{TCPRouteListenerSetIndex: lsKey}); err != nil {
			return nil, err
		}
		resolved.listenerSetTCPRoutes[lsKey] = lsTCPRouteList.Items

		var lsUDPRouteList gatewayv1alpha2.UDPRouteList
		if err := r.Reader.List(ctx, &lsUDPRouteList, client.MatchingFields{UDPRouteListenerSetIndex: lsKey}); err != nil {
			return nil, err
		}
		resolved.listenerSetUDPRoutes[lsKey] = lsUDPRouteList.Items

		for _, tcpRoute := range lsTCPRouteList.Items {
			for _, rule := range tcpRoute.Spec.Rules {
				backends := r.resolveBackendRefs(ctx, rule.BackendRefs, tcpRoute.Namespace, "TCPRoute")
				if len(backends) == 0 {
					continue
				}
				for _, parentRef := range tcpRoute.Spec.ParentRefs {
					if !isListenerSetRef(parentRef) || string(parentRef.Name) != ls.Name ||
						(parentRef.Namespace != nil && string(*parentRef.Namespace) != ls.Namespace) {
						continue
					}
					for _, port := range allowedListenerSetListenerPorts(ctx, r.Reader, &ls, parentRef.SectionName, tcpRoute.Namespace, gw.Namespace) {
						routes = append(routes, proxy.Route{
							Port:     port,
							Protocol: proxy.ProtocolTCP,
							Backends: backends,
						})
					}
				}
			}
		}

		for _, udpRoute := range lsUDPRouteList.Items {
			for _, rule := range udpRoute.Spec.Rules {
				backends := r.resolveBackendRefs(ctx, rule.BackendRefs, udpRoute.Namespace, "UDPRoute")
				if len(backends) == 0 {
					continue
				}
				for _, parentRef := range udpRoute.Spec.ParentRefs {
					if !isListenerSetRef(parentRef) || string(parentRef.Name) != ls.Name ||
						(parentRef.Namespace != nil && string(*parentRef.Namespace) != ls.Namespace) {
						continue
					}
					for _, port := range allowedListenerSetListenerPorts(ctx, r.Reader, &ls, parentRef.SectionName, udpRoute.Namespace, gw.Namespace) {
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

func (r *RouteResolver) OwnedGatewayClassNames(ctx context.Context) ([]string, error) {
	var gatewayClassList gatewayv1.GatewayClassList
	if err := r.Reader.List(ctx, &gatewayClassList); err != nil {
		return nil, err
	}
	var names []string
	for _, gc := range gatewayClassList.Items {
		if gc.Spec.ControllerName == r.ControllerName {
			names = append(names, gc.Name)
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
	for _, ls := range allListenerSets.Items {
		if r.IsListenerSetAllowed(ctx, gateway, &ls) {
			attached = append(attached, ls)
		}
	}

	sort.Slice(attached, func(i int, j int) bool {
		ti := attached[i].CreationTimestamp.Time
		tj := attached[j].CreationTimestamp.Time
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		ki := fmt.Sprintf("%s/%s", attached[i].Namespace, attached[i].Name)
		kj := fmt.Sprintf("%s/%s", attached[j].Namespace, attached[j].Name)
		return ki < kj
	})

	return attached, nil
}

func (r *RouteResolver) IsListenerSetAllowed(ctx context.Context, gateway *gatewayv1.Gateway, ls *gatewayv1.ListenerSet) bool {
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
		return ls.Namespace == gateway.Namespace
	case gatewayv1.NamespacesFromSelector:
		if namespaces.Selector == nil {
			return false
		}
		selector, err := metav1.LabelSelectorAsSelector(namespaces.Selector)
		if err != nil {
			return false
		}
		ns := &corev1.Namespace{}
		if err := r.Reader.Get(ctx, types.NamespacedName{Name: ls.Namespace}, ns); err != nil {
			return false
		}
		return selector.Matches(labels.Set(ns.Labels))
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

func (r *RouteResolver) resolveBackendRefs(ctx context.Context, backendRefs []gatewayv1.BackendRef, routeNamespace string, routeKind string) []proxy.Backend {
	var backends []proxy.Backend
	for _, ref := range backendRefs {
		if ref.Group != nil && *ref.Group != "" {
			continue
		}
		if ref.Kind != nil && *ref.Kind != "Service" {
			continue
		}
		if ref.Port == nil {
			continue
		}

		namespace := routeNamespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}

		if namespace != routeNamespace {
			if !r.isReferenceGranted(ctx, routeNamespace, routeKind, namespace, string(ref.Name)) {
				continue
			}
		}

		backends = append(backends, proxy.Backend{
			Address: fmt.Sprintf("%s.%s.svc.cluster.local", string(ref.Name), namespace),
			Port:    int32(*ref.Port),
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

func allowedListenerPorts(ctx context.Context, c client.Reader, gateway *gatewayv1.Gateway, sectionName *gatewayv1.SectionName, routeNamespace string) []int32 {
	var ports []int32
	for _, listener := range gateway.Spec.Listeners {
		if sectionName != nil && listener.Name != *sectionName {
			continue
		}
		if isRouteAllowedByListener(ctx, c, listener.AllowedRoutes, routeNamespace, gateway.Namespace) {
			ports = append(ports, int32(listener.Port))
		}
	}
	return ports
}

func allowedListenerSetListenerPorts(ctx context.Context, c client.Reader, ls *gatewayv1.ListenerSet, sectionName *gatewayv1.SectionName, routeNamespace string, gatewayNamespace string) []int32 {
	var ports []int32
	for _, entry := range ls.Spec.Listeners {
		if sectionName != nil && entry.Name != *sectionName {
			continue
		}
		if isRouteAllowedByListener(ctx, c, entry.AllowedRoutes, routeNamespace, gatewayNamespace) {
			ports = append(ports, int32(entry.Port))
		}
	}
	return ports
}

func isRouteAllowedByListener(ctx context.Context, c client.Reader, allowedRoutes *gatewayv1.AllowedRoutes, routeNamespace string, ownerNamespace string) bool {
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
		ns := &corev1.Namespace{}
		if err := c.Get(ctx, types.NamespacedName{Name: routeNamespace}, ns); err != nil {
			return false
		}
		return selector.Matches(labels.Set(ns.Labels))
	case gatewayv1.NamespacesFromNone:
		return false
	default:
		return false
	}
}
