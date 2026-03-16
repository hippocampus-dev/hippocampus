package controllers

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gateway-proxy/internal/proxy"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
	GatewayClassNameIndex    = ".spec.gatewayClassName"
	ListenerSetGatewayIndex  = ".spec.parentRef.gateway"
	TCPRouteGatewayIndex     = ".spec.parentRefs.gateway.tcproute"
	UDPRouteGatewayIndex     = ".spec.parentRefs.gateway.udproute"
	TCPRouteListenerSetIndex = ".spec.parentRefs.listenerset.tcproute"
	UDPRouteListenerSetIndex = ".spec.parentRefs.listenerset.udproute"
)

type GatewayController struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ControllerName gatewayv1.GatewayController
	ProxyManager   *proxy.Manager
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

func (r *GatewayController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gateway := &gatewayv1.Gateway{}
	if err := r.Get(ctx, req.NamespacedName, gateway); err != nil {
		if apierrors.IsNotFound(err) {
			routes, _, _, err := r.resolveAllRoutes(ctx)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err := r.ProxyManager.Update(routes); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	owned, err := r.isOwnedGateway(ctx, gateway)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !owned {
		return ctrl.Result{}, nil
	}

	attachedListenerSets, err := r.getAttachedListenerSets(ctx, gateway)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, gateway, attachedListenerSets); err != nil {
		return ctrl.Result{}, err
	}

	routes, conflictedPorts, resolved, err := r.resolveAllRoutes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	proxyError := r.ProxyManager.Update(routes)

	if err := r.updateGatewayStatus(ctx, gateway, attachedListenerSets, conflictedPorts, resolved, proxyError == nil); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateListenerSetStatuses(ctx, gateway, attachedListenerSets, conflictedPorts, resolved); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateRouteStatuses(ctx, gateway, attachedListenerSets, conflictedPorts, resolved); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, proxyError
}

func (r *GatewayController) getAttachedListenerSets(ctx context.Context, gateway *gatewayv1.Gateway) ([]gatewayv1.ListenerSet, error) {
	gatewayKey := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}.String()

	var allListenerSets gatewayv1.ListenerSetList
	if err := r.List(ctx, &allListenerSets, client.MatchingFields{ListenerSetGatewayIndex: gatewayKey}); err != nil {
		return nil, err
	}

	var attached []gatewayv1.ListenerSet
	for _, ls := range allListenerSets.Items {
		if r.isListenerSetAllowed(ctx, gateway, &ls) {
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

func (r *GatewayController) isOwnedGateway(ctx context.Context, gateway *gatewayv1.Gateway) (bool, error) {
	gatewayClass := &gatewayv1.GatewayClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: string(gateway.Spec.GatewayClassName)}, gatewayClass); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return gatewayClass.Spec.ControllerName == r.ControllerName, nil
}

func (r *GatewayController) isListenerSetAllowed(ctx context.Context, gateway *gatewayv1.Gateway, ls *gatewayv1.ListenerSet) bool {
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
		if err := r.Get(ctx, types.NamespacedName{Name: ls.Namespace}, ns); err != nil {
			return false
		}
		return selector.Matches(labels.Set(ns.Labels))
	case gatewayv1.NamespacesFromNone:
		return false
	default:
		return false
	}
}

func (r *GatewayController) reconcileService(ctx context.Context, gateway *gatewayv1.Gateway, attachedListenerSets []gatewayv1.ListenerSet) error {
	var ports []corev1.ServicePort
	for _, l := range gateway.Spec.Listeners {
		protocol := corev1.ProtocolTCP
		if l.Protocol == gatewayv1.UDPProtocolType {
			protocol = corev1.ProtocolUDP
		}
		ports = append(ports, corev1.ServicePort{
			Name:       fmt.Sprintf("%s-%d", strings.ToLower(string(protocol)), l.Port),
			Port:       int32(l.Port),
			TargetPort: intstr.FromInt32(int32(l.Port)),
			Protocol:   protocol,
		})
	}
	for _, ls := range attachedListenerSets {
		for _, entry := range ls.Spec.Listeners {
			protocol := corev1.ProtocolTCP
			if entry.Protocol == gatewayv1.UDPProtocolType {
				protocol = corev1.ProtocolUDP
			}
			ports = append(ports, corev1.ServicePort{
				Name:       fmt.Sprintf("%s-%d", strings.ToLower(string(protocol)), entry.Port),
				Port:       int32(entry.Port),
				TargetPort: intstr.FromInt32(int32(entry.Port)),
				Protocol:   protocol,
			})
		}
	}

	serviceName := gateway.Name
	service := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: gateway.Namespace}, service)

	if apierrors.IsNotFound(err) {
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: gateway.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(gateway, gatewayv1.SchemeGroupVersion.WithKind("Gateway")),
				},
			},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeLoadBalancer,
				Ports: ports,
				Selector: map[string]string{
					"app.kubernetes.io/name":      "gateway-proxy",
					"app.kubernetes.io/component": "",
				},
			},
		}
		if err := r.Create(ctx, service); err != nil {
			return err
		}
		r.Recorder.Eventf(gateway, corev1.EventTypeNormal, "SuccessfulCreated", "Created service: %q", serviceName)
		return nil
	}
	if err != nil {
		return err
	}

	if !portsEqual(service.Spec.Ports, ports) {
		service.Spec.Ports = ports
		if err := r.Update(ctx, service); err != nil {
			return err
		}
		r.Recorder.Eventf(gateway, corev1.EventTypeNormal, "SuccessfulUpdated", "Updated service: %q", serviceName)
	}
	return nil
}

func (r *GatewayController) ownedGatewayClassNames(ctx context.Context) ([]string, error) {
	var gatewayClassList gatewayv1.GatewayClassList
	if err := r.List(ctx, &gatewayClassList); err != nil {
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

func (r *GatewayController) resolveAllRoutes(ctx context.Context) ([]proxy.Route, map[portKey]bool, *resolvedRoutes, error) {
	classNames, err := r.ownedGatewayClassNames(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	var gatewayList gatewayv1.GatewayList
	for _, className := range classNames {
		var list gatewayv1.GatewayList
		if err := r.List(ctx, &list, client.MatchingFields{GatewayClassNameIndex: className}); err != nil {
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
		gatewayKey := types.NamespacedName{Name: gw.Name, Namespace: gw.Namespace}.String()

		var tcpRouteList gatewayv1alpha2.TCPRouteList
		if err := r.List(ctx, &tcpRouteList, client.MatchingFields{TCPRouteGatewayIndex: gatewayKey}); err != nil {
			return nil, nil, nil, err
		}
		resolved.gatewayTCPRoutes[gatewayKey] = tcpRouteList.Items

		var udpRouteList gatewayv1alpha2.UDPRouteList
		if err := r.List(ctx, &udpRouteList, client.MatchingFields{UDPRouteGatewayIndex: gatewayKey}); err != nil {
			return nil, nil, nil, err
		}
		resolved.gatewayUDPRoutes[gatewayKey] = udpRouteList.Items

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
					for _, port := range allowedListenerPorts(ctx, r.Client, &gw, parentRef.SectionName, tcpRoute.Namespace) {
						allRoutes = append(allRoutes, proxy.Route{
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
					for _, port := range allowedListenerPorts(ctx, r.Client, &gw, parentRef.SectionName, udpRoute.Namespace) {
						allRoutes = append(allRoutes, proxy.Route{
							Port:     port,
							Protocol: proxy.ProtocolUDP,
							Backends: backends,
						})
					}
				}
			}
		}

		attachedListenerSets, err := r.getAttachedListenerSets(ctx, &gw)
		if err != nil {
			return nil, nil, nil, err
		}

		for _, ls := range attachedListenerSets {
			lsKey := types.NamespacedName{Name: ls.Name, Namespace: ls.Namespace}.String()

			var lsTCPRouteList gatewayv1alpha2.TCPRouteList
			if err := r.List(ctx, &lsTCPRouteList, client.MatchingFields{TCPRouteListenerSetIndex: lsKey}); err != nil {
				return nil, nil, nil, err
			}
			resolved.listenerSetTCPRoutes[lsKey] = lsTCPRouteList.Items

			var lsUDPRouteList gatewayv1alpha2.UDPRouteList
			if err := r.List(ctx, &lsUDPRouteList, client.MatchingFields{UDPRouteListenerSetIndex: lsKey}); err != nil {
				return nil, nil, nil, err
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
						for _, port := range allowedListenerSetListenerPorts(ctx, r.Client, &ls, parentRef.SectionName, tcpRoute.Namespace, gw.Namespace) {
							allRoutes = append(allRoutes, proxy.Route{
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
						for _, port := range allowedListenerSetListenerPorts(ctx, r.Client, &ls, parentRef.SectionName, udpRoute.Namespace, gw.Namespace) {
							allRoutes = append(allRoutes, proxy.Route{
								Port:     port,
								Protocol: proxy.ProtocolUDP,
								Backends: backends,
							})
						}
					}
				}
			}
		}
	}

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

func (r *GatewayController) updateGatewayStatus(ctx context.Context, gateway *gatewayv1.Gateway, attachedListenerSets []gatewayv1.ListenerSet, conflictedPorts map[portKey]bool, resolved *resolvedRoutes, programmed bool) error {
	hasConflictedListener := false
	for _, listener := range gateway.Spec.Listeners {
		if conflictedPorts[listenerPortKey(listener)] {
			hasConflictedListener = true
			break
		}
	}

	acceptedReason := gatewayv1.GatewayReasonAccepted
	if hasConflictedListener {
		acceptedReason = gatewayv1.GatewayReasonListenersNotValid
	}
	apimeta.SetStatusCondition(&gateway.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: gateway.Generation,
		Reason:             string(acceptedReason),
	})

	programmedStatus := metav1.ConditionTrue
	programmedReason := gatewayv1.GatewayReasonProgrammed
	if !programmed {
		programmedStatus = metav1.ConditionFalse
		programmedReason = gatewayv1.GatewayReasonAddressNotUsable
	}
	apimeta.SetStatusCondition(&gateway.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             programmedStatus,
		ObservedGeneration: gateway.Generation,
		Reason:             string(programmedReason),
	})

	gatewayKey := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}.String()
	now := metav1.Now()
	var listenerStatuses []gatewayv1.ListenerStatus
	for _, listener := range gateway.Spec.Listeners {
		conflictedCondition := metav1.Condition{
			Type:               string(gatewayv1.ListenerConditionConflicted),
			ObservedGeneration: gateway.Generation,
			LastTransitionTime: now,
		}
		lKey := listenerPortKey(listener)
		if conflictedPorts[lKey] {
			conflictedCondition.Status = metav1.ConditionTrue
			conflictedCondition.Reason = string(gatewayv1.ListenerReasonProtocolConflict)
			conflictedCondition.Message = fmt.Sprintf("port %d is used by multiple routes", listener.Port)
			r.Recorder.Eventf(gateway, corev1.EventTypeWarning, "PortConflict", "listener %q port %d conflicts with another route", listener.Name, listener.Port)
		} else {
			conflictedCondition.Status = metav1.ConditionFalse
			conflictedCondition.Reason = string(gatewayv1.ListenerReasonNoConflicts)
		}
		acceptedCondition := metav1.Condition{
			Type:               string(gatewayv1.ListenerConditionAccepted),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: gateway.Generation,
			LastTransitionTime: now,
			Reason:             string(gatewayv1.ListenerReasonAccepted),
		}
		programmedCondition := metav1.Condition{
			Type:               string(gatewayv1.ListenerConditionProgrammed),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: gateway.Generation,
			LastTransitionTime: now,
			Reason:             string(gatewayv1.ListenerReasonProgrammed),
		}
		resolvedRefsCondition := metav1.Condition{
			Type:               string(gatewayv1.ListenerConditionResolvedRefs),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: gateway.Generation,
			LastTransitionTime: now,
			Reason:             string(gatewayv1.ListenerReasonResolvedRefs),
		}
		var attachedRoutes int32
		for _, route := range resolved.gatewayTCPRoutes[gatewayKey] {
			for _, ref := range route.Spec.ParentRefs {
				if isGatewayRef(ref) && string(ref.Name) == gateway.Name &&
					(ref.Namespace == nil || string(*ref.Namespace) == gateway.Namespace) &&
					(ref.SectionName == nil || *ref.SectionName == listener.Name) &&
					isRouteAllowedByListener(ctx, r.Client, listener.AllowedRoutes, route.Namespace, gateway.Namespace) {
					if !conflictedPorts[lKey] {
						attachedRoutes++
						break
					}
				}
			}
		}
		for _, route := range resolved.gatewayUDPRoutes[gatewayKey] {
			for _, ref := range route.Spec.ParentRefs {
				if isGatewayRef(ref) && string(ref.Name) == gateway.Name &&
					(ref.Namespace == nil || string(*ref.Namespace) == gateway.Namespace) &&
					(ref.SectionName == nil || *ref.SectionName == listener.Name) &&
					isRouteAllowedByListener(ctx, r.Client, listener.AllowedRoutes, route.Namespace, gateway.Namespace) {
					if !conflictedPorts[lKey] {
						attachedRoutes++
						break
					}
				}
			}
		}
		listenerStatuses = append(listenerStatuses, gatewayv1.ListenerStatus{
			Name:           listener.Name,
			AttachedRoutes: attachedRoutes,
			Conditions:     []metav1.Condition{acceptedCondition, programmedCondition, resolvedRefsCondition, conflictedCondition},
		})
	}
	gateway.Status.Listeners = listenerStatuses

	attachedCount := int32(len(attachedListenerSets))
	gateway.Status.AttachedListenerSets = &attachedCount

	return r.Status().Update(ctx, gateway)
}

func (r *GatewayController) updateListenerSetStatuses(ctx context.Context, gateway *gatewayv1.Gateway, attachedListenerSets []gatewayv1.ListenerSet, conflictedPorts map[portKey]bool, resolved *resolvedRoutes) error {
	now := metav1.Now()

	for i := range attachedListenerSets {
		ls := &attachedListenerSets[i]
		lsKey := types.NamespacedName{Name: ls.Name, Namespace: ls.Namespace}.String()

		apimeta.SetStatusCondition(&ls.Status.Conditions, metav1.Condition{
			Type:               string(gatewayv1.ListenerSetConditionAccepted),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: ls.Generation,
			Reason:             string(gatewayv1.ListenerSetReasonAccepted),
		})
		apimeta.SetStatusCondition(&ls.Status.Conditions, metav1.Condition{
			Type:               string(gatewayv1.ListenerSetConditionProgrammed),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: ls.Generation,
			Reason:             string(gatewayv1.ListenerSetReasonProgrammed),
		})

		var listenerStatuses []gatewayv1.ListenerEntryStatus
		for _, entry := range ls.Spec.Listeners {
			conflictedCondition := metav1.Condition{
				Type:               string(gatewayv1.ListenerEntryConditionConflicted),
				ObservedGeneration: ls.Generation,
				LastTransitionTime: now,
			}
			eKey := listenerEntryPortKey(entry)
			if conflictedPorts[eKey] {
				conflictedCondition.Status = metav1.ConditionTrue
				conflictedCondition.Reason = string(gatewayv1.ListenerEntryReasonProtocolConflict)
				conflictedCondition.Message = fmt.Sprintf("port %d is used by multiple routes", entry.Port)
			} else {
				conflictedCondition.Status = metav1.ConditionFalse
				conflictedCondition.Reason = "NoConflicts"
			}
			var attachedRoutes int32
			for _, route := range resolved.listenerSetTCPRoutes[lsKey] {
				for _, ref := range route.Spec.ParentRefs {
					if isListenerSetRef(ref) && string(ref.Name) == ls.Name &&
						(ref.Namespace == nil || string(*ref.Namespace) == ls.Namespace) &&
						(ref.SectionName == nil || *ref.SectionName == entry.Name) &&
						isRouteAllowedByListener(ctx, r.Client, entry.AllowedRoutes, route.Namespace, gateway.Namespace) {
						if !conflictedPorts[eKey] {
							attachedRoutes++
							break
						}
					}
				}
			}
			for _, route := range resolved.listenerSetUDPRoutes[lsKey] {
				for _, ref := range route.Spec.ParentRefs {
					if isListenerSetRef(ref) && string(ref.Name) == ls.Name &&
						(ref.Namespace == nil || string(*ref.Namespace) == ls.Namespace) &&
						(ref.SectionName == nil || *ref.SectionName == entry.Name) &&
						isRouteAllowedByListener(ctx, r.Client, entry.AllowedRoutes, route.Namespace, gateway.Namespace) {
						if !conflictedPorts[eKey] {
							attachedRoutes++
							break
						}
					}
				}
			}
			listenerStatuses = append(listenerStatuses, gatewayv1.ListenerEntryStatus{
				Name:           entry.Name,
				AttachedRoutes: attachedRoutes,
				Conditions:     []metav1.Condition{conflictedCondition},
			})
		}
		ls.Status.Listeners = listenerStatuses

		if err := r.Status().Update(ctx, ls); err != nil {
			return err
		}
	}

	var rejectedListenerSets gatewayv1.ListenerSetList
	gatewayKey := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}.String()
	if err := r.List(ctx, &rejectedListenerSets, client.MatchingFields{ListenerSetGatewayIndex: gatewayKey}); err != nil {
		return err
	}

	attachedNames := make(map[string]bool)
	for _, ls := range attachedListenerSets {
		attachedNames[fmt.Sprintf("%s/%s", ls.Namespace, ls.Name)] = true
	}

	for i := range rejectedListenerSets.Items {
		ls := &rejectedListenerSets.Items[i]
		key := fmt.Sprintf("%s/%s", ls.Namespace, ls.Name)
		if attachedNames[key] {
			continue
		}

		apimeta.SetStatusCondition(&ls.Status.Conditions, metav1.Condition{
			Type:               string(gatewayv1.ListenerSetConditionAccepted),
			Status:             metav1.ConditionFalse,
			ObservedGeneration: ls.Generation,
			Reason:             string(gatewayv1.ListenerSetReasonNotAllowed),
			Message:            "ListenerSet is not allowed by the Gateway's allowedListeners configuration",
		})
		apimeta.SetStatusCondition(&ls.Status.Conditions, metav1.Condition{
			Type:               string(gatewayv1.ListenerSetConditionProgrammed),
			Status:             metav1.ConditionFalse,
			ObservedGeneration: ls.Generation,
			Reason:             string(gatewayv1.ListenerSetReasonNotAllowed),
		})

		if err := r.Status().Update(ctx, ls); err != nil {
			return err
		}
	}

	return nil
}

func (r *GatewayController) updateRouteStatuses(ctx context.Context, gateway *gatewayv1.Gateway, attachedListenerSets []gatewayv1.ListenerSet, conflictedPorts map[portKey]bool, resolved *resolvedRoutes) error {
	controllerName := r.ControllerName
	now := metav1.Now()
	gatewayKey := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}.String()

	for i := range resolved.gatewayTCPRoutes[gatewayKey] {
		tcpRoute := &resolved.gatewayTCPRoutes[gatewayKey][i]
		refsResolved, failureReason := r.checkTCPRouteBackendRefs(ctx, tcpRoute)
		statuses := routeParentStatuses(ctx, r.Client, gateway, tcpRoute.Spec.ParentRefs, tcpRoute.Namespace, proxy.ProtocolTCP, tcpRoute.Generation, controllerName, conflictedPorts, refsResolved, failureReason, now)
		tcpRoute.Status.Parents = mergeRouteParentStatuses(tcpRoute.Status.Parents, statuses, controllerName)
		if err := r.Status().Update(ctx, tcpRoute); err != nil {
			return err
		}
	}

	for i := range resolved.gatewayUDPRoutes[gatewayKey] {
		udpRoute := &resolved.gatewayUDPRoutes[gatewayKey][i]
		refsResolved, failureReason := r.checkUDPRouteBackendRefs(ctx, udpRoute)
		statuses := routeParentStatuses(ctx, r.Client, gateway, udpRoute.Spec.ParentRefs, udpRoute.Namespace, proxy.ProtocolUDP, udpRoute.Generation, controllerName, conflictedPorts, refsResolved, failureReason, now)
		udpRoute.Status.Parents = mergeRouteParentStatuses(udpRoute.Status.Parents, statuses, controllerName)
		if err := r.Status().Update(ctx, udpRoute); err != nil {
			return err
		}
	}

	for _, ls := range attachedListenerSets {
		lsKey := types.NamespacedName{Name: ls.Name, Namespace: ls.Namespace}.String()

		for i := range resolved.listenerSetTCPRoutes[lsKey] {
			tcpRoute := &resolved.listenerSetTCPRoutes[lsKey][i]
			refsResolved, failureReason := r.checkTCPRouteBackendRefs(ctx, tcpRoute)
			statuses := listenerSetRouteParentStatuses(ctx, r.Client, &ls, tcpRoute.Spec.ParentRefs, tcpRoute.Namespace, gateway.Namespace, proxy.ProtocolTCP, tcpRoute.Generation, controllerName, conflictedPorts, refsResolved, failureReason, now)
			tcpRoute.Status.Parents = mergeRouteParentStatuses(tcpRoute.Status.Parents, statuses, controllerName)
			if err := r.Status().Update(ctx, tcpRoute); err != nil {
				return err
			}
		}

		for i := range resolved.listenerSetUDPRoutes[lsKey] {
			udpRoute := &resolved.listenerSetUDPRoutes[lsKey][i]
			refsResolved, failureReason := r.checkUDPRouteBackendRefs(ctx, udpRoute)
			statuses := listenerSetRouteParentStatuses(ctx, r.Client, &ls, udpRoute.Spec.ParentRefs, udpRoute.Namespace, gateway.Namespace, proxy.ProtocolUDP, udpRoute.Generation, controllerName, conflictedPorts, refsResolved, failureReason, now)
			udpRoute.Status.Parents = mergeRouteParentStatuses(udpRoute.Status.Parents, statuses, controllerName)
			if err := r.Status().Update(ctx, udpRoute); err != nil {
				return err
			}
		}
	}

	return nil
}

func routeParentStatuses(ctx context.Context, c client.Client, gateway *gatewayv1.Gateway, parentRefs []gatewayv1.ParentReference, routeNamespace string, routeProtocol proxy.Protocol, generation int64, controllerName gatewayv1.GatewayController, conflictedPorts map[portKey]bool, refsResolved bool, failureReason string, now metav1.Time) []gatewayv1.RouteParentStatus {
	var statuses []gatewayv1.RouteParentStatus
	for _, ref := range parentRefs {
		if !isGatewayRef(ref) {
			continue
		}
		if string(ref.Name) != gateway.Name {
			continue
		}
		if ref.Namespace != nil && string(*ref.Namespace) != gateway.Namespace {
			continue
		}

		ports := allowedListenerPorts(ctx, c, gateway, ref.SectionName, routeNamespace)
		accepted := routeAcceptedCondition(ports, routeProtocol, conflictedPorts, generation, now)

		resolvedRefs := routeResolvedRefsCondition(refsResolved, failureReason, generation, now)

		statuses = append(statuses, gatewayv1.RouteParentStatus{
			ParentRef:      ref,
			ControllerName: controllerName,
			Conditions:     []metav1.Condition{accepted, resolvedRefs},
		})
	}
	return statuses
}

func listenerSetRouteParentStatuses(ctx context.Context, c client.Client, ls *gatewayv1.ListenerSet, parentRefs []gatewayv1.ParentReference, routeNamespace string, gatewayNamespace string, routeProtocol proxy.Protocol, generation int64, controllerName gatewayv1.GatewayController, conflictedPorts map[portKey]bool, refsResolved bool, failureReason string, now metav1.Time) []gatewayv1.RouteParentStatus {
	var statuses []gatewayv1.RouteParentStatus
	for _, ref := range parentRefs {
		if !isListenerSetRef(ref) {
			continue
		}
		if string(ref.Name) != ls.Name {
			continue
		}
		if ref.Namespace != nil && string(*ref.Namespace) != ls.Namespace {
			continue
		}

		ports := allowedListenerSetListenerPorts(ctx, c, ls, ref.SectionName, routeNamespace, gatewayNamespace)
		accepted := routeAcceptedCondition(ports, routeProtocol, conflictedPorts, generation, now)

		resolvedRefs := routeResolvedRefsCondition(refsResolved, failureReason, generation, now)

		statuses = append(statuses, gatewayv1.RouteParentStatus{
			ParentRef:      ref,
			ControllerName: controllerName,
			Conditions:     []metav1.Condition{accepted, resolvedRefs},
		})
	}
	return statuses
}

func routeAcceptedCondition(ports []int32, routeProtocol proxy.Protocol, conflictedPorts map[portKey]bool, generation int64, now metav1.Time) metav1.Condition {
	accepted := metav1.Condition{
		Type:               string(gatewayv1.RouteConditionAccepted),
		ObservedGeneration: generation,
		LastTransitionTime: now,
	}
	if len(ports) == 0 {
		accepted.Status = metav1.ConditionFalse
		accepted.Reason = string(gatewayv1.RouteReasonNoMatchingParent)
		return accepted
	}
	for _, port := range ports {
		if conflictedPorts[portKey{Port: port, Protocol: routeProtocol}] {
			accepted.Status = metav1.ConditionFalse
			accepted.Reason = string(gatewayv1.RouteReasonUnsupportedValue)
			accepted.Message = fmt.Sprintf("port %d is used by multiple routes", port)
			return accepted
		}
	}
	accepted.Status = metav1.ConditionTrue
	accepted.Reason = string(gatewayv1.RouteReasonAccepted)
	return accepted
}

func routeResolvedRefsCondition(refsResolved bool, failureReason string, generation int64, now metav1.Time) metav1.Condition {
	resolvedRefs := metav1.Condition{
		Type:               string(gatewayv1.RouteConditionResolvedRefs),
		ObservedGeneration: generation,
		LastTransitionTime: now,
	}
	if refsResolved {
		resolvedRefs.Status = metav1.ConditionTrue
		resolvedRefs.Reason = string(gatewayv1.RouteReasonResolvedRefs)
	} else {
		resolvedRefs.Status = metav1.ConditionFalse
		resolvedRefs.Reason = failureReason
	}
	return resolvedRefs
}

func mergeRouteParentStatuses(existing []gatewayv1.RouteParentStatus, updated []gatewayv1.RouteParentStatus, controllerName gatewayv1.GatewayController) []gatewayv1.RouteParentStatus {
	var merged []gatewayv1.RouteParentStatus
	for _, s := range existing {
		if s.ControllerName != controllerName {
			merged = append(merged, s)
		}
	}
	return append(merged, updated...)
}

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

func allowedListenerPorts(ctx context.Context, c client.Client, gateway *gatewayv1.Gateway, sectionName *gatewayv1.SectionName, routeNamespace string) []int32 {
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

func allowedListenerSetListenerPorts(ctx context.Context, c client.Client, ls *gatewayv1.ListenerSet, sectionName *gatewayv1.SectionName, routeNamespace string, gatewayNamespace string) []int32 {
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

func isRouteAllowedByListener(ctx context.Context, c client.Client, allowedRoutes *gatewayv1.AllowedRoutes, routeNamespace string, ownerNamespace string) bool {
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
	default:
		return false
	}
}

func listenerPortKey(l gatewayv1.Listener) portKey {
	p := proxy.ProtocolTCP
	if l.Protocol == gatewayv1.UDPProtocolType {
		p = proxy.ProtocolUDP
	}
	return portKey{Port: int32(l.Port), Protocol: p}
}

func listenerEntryPortKey(e gatewayv1.ListenerEntry) portKey {
	p := proxy.ProtocolTCP
	if e.Protocol == gatewayv1.UDPProtocolType {
		p = proxy.ProtocolUDP
	}
	return portKey{Port: int32(e.Port), Protocol: p}
}

func portsEqual(a []corev1.ServicePort, b []corev1.ServicePort) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Port != b[i].Port ||
			a[i].TargetPort != b[i].TargetPort || a[i].Protocol != b[i].Protocol {
			return false
		}
	}
	return true
}

func (r *GatewayController) resolveBackendRefs(ctx context.Context, backendRefs []gatewayv1.BackendRef, routeNamespace string, routeKind string) []proxy.Backend {
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

func (r *GatewayController) checkTCPRouteBackendRefs(ctx context.Context, route *gatewayv1alpha2.TCPRoute) (bool, string) {
	for _, rule := range route.Spec.Rules {
		if resolved, reason := r.checkBackendRefs(ctx, rule.BackendRefs, route.Namespace, "TCPRoute"); !resolved {
			return false, reason
		}
	}
	return true, ""
}

func (r *GatewayController) checkUDPRouteBackendRefs(ctx context.Context, route *gatewayv1alpha2.UDPRoute) (bool, string) {
	for _, rule := range route.Spec.Rules {
		if resolved, reason := r.checkBackendRefs(ctx, rule.BackendRefs, route.Namespace, "UDPRoute"); !resolved {
			return false, reason
		}
	}
	return true, ""
}

func (r *GatewayController) checkBackendRefs(ctx context.Context, backendRefs []gatewayv1.BackendRef, routeNamespace string, routeKind string) (bool, string) {
	for _, ref := range backendRefs {
		if ref.Group != nil && *ref.Group != "" {
			return false, string(gatewayv1.RouteReasonInvalidKind)
		}
		if ref.Kind != nil && *ref.Kind != "Service" {
			return false, string(gatewayv1.RouteReasonInvalidKind)
		}
		if ref.Port == nil {
			return false, string(gatewayv1.RouteReasonUnsupportedValue)
		}
		namespace := routeNamespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}
		if namespace != routeNamespace {
			if !r.isReferenceGranted(ctx, routeNamespace, routeKind, namespace, string(ref.Name)) {
				return false, string(gatewayv1.RouteReasonRefNotPermitted)
			}
		}
		service := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{Name: string(ref.Name), Namespace: namespace}, service); err != nil {
			return false, string(gatewayv1.RouteReasonBackendNotFound)
		}
		portFound := false
		for _, sp := range service.Spec.Ports {
			if sp.Port == int32(*ref.Port) {
				portFound = true
				break
			}
		}
		if !portFound {
			return false, string(gatewayv1.RouteReasonBackendNotFound)
		}
	}
	return true, ""
}

func (r *GatewayController) isReferenceGranted(ctx context.Context, fromNamespace string, fromKind string, toNamespace string, toName string) bool {
	var referenceGrants gatewayv1.ReferenceGrantList
	if err := r.List(ctx, &referenceGrants, client.InNamespace(toNamespace)); err != nil {
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

func (r *GatewayController) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()

	if err := mgr.GetFieldIndexer().IndexField(ctx, &gatewayv1.Gateway{}, GatewayClassNameIndex, func(obj client.Object) []string {
		gw := obj.(*gatewayv1.Gateway)
		return []string{string(gw.Spec.GatewayClassName)}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &gatewayv1.ListenerSet{}, ListenerSetGatewayIndex, func(obj client.Object) []string {
		ls := obj.(*gatewayv1.ListenerSet)
		ref := ls.Spec.ParentRef
		namespace := ls.Namespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}
		return []string{types.NamespacedName{
			Namespace: namespace,
			Name:      string(ref.Name),
		}.String()}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &gatewayv1alpha2.TCPRoute{}, TCPRouteGatewayIndex, func(obj client.Object) []string {
		route := obj.(*gatewayv1alpha2.TCPRoute)
		return gatewayIndexFunc(route.Spec.ParentRefs, route.Namespace)
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &gatewayv1alpha2.UDPRoute{}, UDPRouteGatewayIndex, func(obj client.Object) []string {
		route := obj.(*gatewayv1alpha2.UDPRoute)
		return gatewayIndexFunc(route.Spec.ParentRefs, route.Namespace)
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &gatewayv1alpha2.TCPRoute{}, TCPRouteListenerSetIndex, func(obj client.Object) []string {
		route := obj.(*gatewayv1alpha2.TCPRoute)
		return listenerSetIndexFunc(route.Spec.ParentRefs, route.Namespace)
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &gatewayv1alpha2.UDPRoute{}, UDPRouteListenerSetIndex, func(obj client.Object) []string {
		route := obj.(*gatewayv1alpha2.UDPRoute)
		return listenerSetIndexFunc(route.Spec.ParentRefs, route.Namespace)
	}); err != nil {
		return err
	}

	mapListenerSetToGateway := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		ls := obj.(*gatewayv1.ListenerSet)
		ref := ls.Spec.ParentRef
		namespace := ls.Namespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}
		return []ctrl.Request{{
			NamespacedName: types.NamespacedName{
				Name:      string(ref.Name),
				Namespace: namespace,
			},
		}}
	})

	mapTCPRouteToGateway := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		route := obj.(*gatewayv1alpha2.TCPRoute)
		return routeParentGateways(ctx, mgr.GetClient(), route.Spec.ParentRefs, route.Namespace)
	})

	mapUDPRouteToGateway := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		route := obj.(*gatewayv1alpha2.UDPRoute)
		return routeParentGateways(ctx, mgr.GetClient(), route.Spec.ParentRefs, route.Namespace)
	})

	mapReferenceGrantToGateway := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		var requests []ctrl.Request
		classNames, err := r.ownedGatewayClassNames(ctx)
		if err != nil {
			return nil
		}
		for _, className := range classNames {
			var list gatewayv1.GatewayList
			if err := mgr.GetClient().List(ctx, &list, client.MatchingFields{GatewayClassNameIndex: className}); err != nil {
				return nil
			}
			for _, gw := range list.Items {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      gw.Name,
						Namespace: gw.Namespace,
					},
				})
			}
		}
		return requests
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}).
		Owns(&corev1.Service{}).
		Watches(&gatewayv1.ListenerSet{}, mapListenerSetToGateway).
		Watches(&gatewayv1alpha2.TCPRoute{}, mapTCPRouteToGateway).
		Watches(&gatewayv1alpha2.UDPRoute{}, mapUDPRouteToGateway).
		Watches(&gatewayv1.ReferenceGrant{}, mapReferenceGrantToGateway).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

func routeParentGateways(ctx context.Context, c client.Client, parentRefs []gatewayv1.ParentReference, routeNamespace string) []ctrl.Request {
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
		} else if isListenerSetRef(ref) {
			namespace := routeNamespace
			if ref.Namespace != nil {
				namespace = string(*ref.Namespace)
			}
			ls := &gatewayv1.ListenerSet{}
			if err := c.Get(ctx, types.NamespacedName{Name: string(ref.Name), Namespace: namespace}, ls); err != nil {
				continue
			}
			gwNamespace := ls.Namespace
			if ls.Spec.ParentRef.Namespace != nil {
				gwNamespace = string(*ls.Spec.ParentRef.Namespace)
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      string(ls.Spec.ParentRef.Name),
					Namespace: gwNamespace,
				},
			})
		}
	}
	return requests
}
