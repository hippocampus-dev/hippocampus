# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gateway-proxy is a Kubernetes controller that watches Gateway API resources (Gateway, ListenerSet, TCPRoute, UDPRoute) and dynamically manages TCP/UDP proxy listeners to expose multiple ports through a single LoadBalancer Service. It eliminates the need to create separate LoadBalancer Services for each TCP/UDP port. ListenerSet (GEP-1713) support enables decentralized listener ownership: one team creates a Gateway, and other teams independently add ports via ListenerSets without modifying the Gateway.

## Common Development Commands

```bash
make dev    # Run with Skaffold for hot-reload development (includes port-forwarding)
```

## High-Level Architecture

The application consists of five main components:

1. **GatewayClass Controller** (`internal/controllers/gatewayclass_controller.go`)
   - Watches GatewayClass resources and sets Accepted and SupportedVersion status conditions for classes matching the controller's `controllerName` (`kaidotio.github.io/gateway-proxy`)

2. **Gateway Controller** (`internal/controllers/gateway_controller.go`)
   - Reconciles Gateway objects whose GatewayClass has a matching `controllerName`
   - Discovers ListenerSets via parentRef, filters by Gateway's `allowedListeners` policy (All/Same/Selector/None), sorts by creationTimestamp then namespace/name
   - Merges Gateway listeners and allowed ListenerSet listeners into a unified listener list
   - Resolves TCPRoute and UDPRoute resources referencing either the Gateway or its ListenerSets using field indexers on parentRefs
   - Detects port conflicts when multiple routes bind to the same port; conflicted routes are excluded from proxying
   - Creates and manages a `{gateway-name}` LoadBalancer Service with ports named `{protocol}-{port}` (e.g., `tcp-5353`, `udp-53`) derived from merged listeners
   - Updates Gateway status conditions (Accepted with ListenersNotValid reason when listeners conflict, Programmed with AddressNotUsable on bind failure) using `apimeta.SetStatusCondition` for idempotent transitions, and per-listener status conditions (Accepted, Programmed, ResolvedRefs, Conflicted) with per-listener AttachedRoutes count, and AttachedListenerSets count
   - Updates ListenerSet status conditions (Accepted, Programmed) and per-listener entry status conditions (Conflicted) with per-entry AttachedRoutes count, and marks rejected ListenerSets as NotAllowed
   - Filters routes by per-listener AllowedRoutes namespace policy (All/Same/Selector, default: Same) so routes from disallowed namespaces are not attached
   - When a route's parentRef has sectionName=nil, attaches to all matching listeners (not just the first)
   - Updates Route status on TCPRoute and UDPRoute resources (Accepted, ResolvedRefs conditions via RouteParentStatus); validates backendRefs for unsupported kinds (InvalidKind), missing port (UnsupportedValue), backend Service existence (BackendNotFound), port existence on Service, and cross-namespace ReferenceGrant (RefNotPermitted)
   - Watches ListenerSet, TCPRoute, UDPRoute, and ReferenceGrant changes and maps them back to affected Gateways via parentRefs; uses `GenerationChangedPredicate` to prevent status-update-triggered reconcile loops

3. **Gateway Webhook** (`internal/handler/gateway_handler.go`)
   - Validating admission webhook that rejects Gateway or ListenerSet create/update when listener ports conflict with existing Gateways and their allowed ListenerSets
   - Filters ListenerSets by Gateway's `allowedListeners` policy (All/Same/Selector/None) when collecting used ports, so disallowed ListenerSets do not cause false port conflicts
   - Filters by GatewayClass `controllerName` to only validate relevant Gateways

4. **Proxy Manager** (`internal/proxy/manager.go`)
   - Manages lifecycle of per-port per-protocol TCP and UDP listeners (keyed by port+protocol, so TCP:53 and UDP:53 coexist)
   - Compares desired routes against running listeners, starting/stopping as needed
   - Thread-safe via mutex-protected listener map
   - Update() returns error on bind failure; closes replaced listeners synchronously before rebinding to avoid port conflicts, then binds listeners via startTCP/startUDP; returns early on the first bind failure (readyz becomes false, so the Pod receives no traffic and remaining listeners are unnecessary)
   - Exposes Ready() for readyz probe; becomes ready after Update() completes without bind failures
   - Bind failures are propagated to the controller, which sets Gateway Programmed=False with AddressNotUsable reason

5. **TCP/UDP Proxying** (`internal/proxy/tcp.go`, `internal/proxy/udp.go`, `internal/proxy/proxyprotocol.go`)
   - TCP: Accepts connections and proxies bidirectionally to backend services using io.Copy
   - TCP: Optionally prepends PROXY protocol header (`--proxy-protocol-version 1` for v1 text, `--proxy-protocol-version 2` for v2 binary) so backends can identify the original client IP
   - UDP: Forwards datagrams to backend services with session tracking and idle timeout
   - UDP: Optionally prepends PROXY protocol v2 binary header per datagram (`--proxy-protocol-version 2`); v1 is not supported for UDP

Key design decisions:
- Uses controller-runtime for Kubernetes integration
- Watches Gateway API resources including ListenerSet (GEP-1713) for decentralized listener ownership
- Single LoadBalancer Service per Gateway, with ports dynamically managed
- Owner references ensure Service cleanup when Gateway is deleted
- MaxConcurrentReconciles set to 1 to avoid race conditions on shared proxy state
- Port conflict detection considers both port number and protocol (TCP:53 and UDP:53 can coexist)
- Dual-layer port conflict protection: webhook rejects at admission, controller detects at reconciliation
- Readyz probe gates on proxy listener bind completion, preventing traffic before listeners are ready
