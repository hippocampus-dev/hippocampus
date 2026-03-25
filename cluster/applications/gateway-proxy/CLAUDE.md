# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gateway-proxy is a Kubernetes controller that watches Gateway API resources (Gateway, ListenerSet, TCPRoute, UDPRoute) and dynamically manages TCP/UDP proxy listeners to expose multiple ports through a single LoadBalancer Service. It eliminates the need to create separate LoadBalancer Services for each TCP/UDP port. ListenerSet (GEP-1713) support enables decentralized listener ownership: one team creates a Gateway, and other teams independently add ports via ListenerSets without modifying the Gateway.

## Common Development Commands

```bash
make dev    # Run with Skaffold for hot-reload development (includes port-forwarding)
```

## High-Level Architecture

The application consists of seven main components:

1. **GatewayClass Controller** (`internal/controllers/gatewayclass_controller.go`)
   - Watches GatewayClass resources and sets Accepted and SupportedVersion status conditions for classes matching the controller's `controllerName` (`kaidotio.github.io/gateway-proxy`)

2. **Gateway Controller** (`internal/controllers/gateway_controller.go`)
   - Reconciles Gateway objects whose GatewayClass has a matching `controllerName`
   - Creates and manages a `{gateway-name}` LoadBalancer Service with ports named `{protocol}-{port}` (e.g., `tcp-5353`, `udp-53`) derived from merged Gateway and ListenerSet listeners
   - Updates Gateway status conditions (Accepted with ListenersNotValid reason when listeners conflict, Programmed with AddressNotUsable when proxy listeners are not ready) using `apimeta.SetStatusCondition` for idempotent transitions, and per-listener status conditions (Accepted, Programmed, ResolvedRefs, Conflicted) with per-listener AttachedRoutes count, and AttachedListenerSets count
   - Updates ListenerSet status conditions (Accepted, Programmed) and per-listener entry status conditions (Conflicted) with per-entry AttachedRoutes count, and marks rejected ListenerSets as NotAllowed
   - Filters routes by per-listener AllowedRoutes namespace policy (All/Same/Selector, default: Same) so routes from disallowed namespaces are not attached
   - When a route's parentRef has sectionName=nil, attaches to all matching listeners (not just the first)
   - Updates Route status on TCPRoute and UDPRoute resources (Accepted, ResolvedRefs conditions via RouteParentStatus); validates backendRefs for unsupported kinds (InvalidKind), missing port (UnsupportedValue), backend Service existence (BackendNotFound), port existence on Service, and cross-namespace ReferenceGrant (RefNotPermitted)
   - Watches ListenerSet, TCPRoute, UDPRoute, and ReferenceGrant changes and maps them back to affected Gateways via parentRefs; uses `GenerationChangedPredicate` to prevent status-update-triggered reconcile loops
   - Uses RouteResolver for route resolution and ProxyManager.Ready() for Programmed status

3. **Route Resolver** (`internal/controllers/route_resolver.go`)
   - Resolves all TCPRoute and UDPRoute resources across all owned Gateways and their attached ListenerSets
   - Discovers ListenerSets via parentRef, filters by Gateway's `allowedListeners` policy (All/Same/Selector/None), sorts by creationTimestamp then namespace/name
   - Resolves backendRefs, validating Service kind, port presence, and cross-namespace ReferenceGrant
   - Detects port conflicts when multiple routes bind to the same port+protocol; conflicted routes are excluded
   - Shared by both GatewayController (for status updates) and ProxyRunner (for proxy listener management)

4. **Proxy Runner** (`internal/controllers/proxy_runner.go`)
   - Runs as a controller-runtime Runnable with `NeedLeaderElection() = false`, so proxy listeners run on all Pods (not just the leader)
   - Watches Gateway API resources via cache informers and triggers proxy reconciliation on any change
   - Uses RouteResolver to resolve routes, then calls ProxyManager.Update() to start/stop listeners
   - Coalesces rapid events via a buffered channel to avoid redundant proxy updates

5. **Gateway Webhook** (`internal/handler/gateway_handler.go`)
   - Validating admission webhook that rejects Gateway or ListenerSet create/update when listener ports conflict with existing Gateways and their allowed ListenerSets
   - Filters ListenerSets by Gateway's `allowedListeners` policy (All/Same/Selector/None) when collecting used ports, so disallowed ListenerSets do not cause false port conflicts
   - Filters by GatewayClass `controllerName` to only validate relevant Gateways

6. **Proxy Manager** (`internal/proxy/manager.go`)
   - Manages lifecycle of per-port per-protocol TCP and UDP listeners (keyed by port+protocol, so TCP:53 and UDP:53 coexist)
   - Compares desired routes against running listeners, starting/stopping as needed
   - Thread-safe via mutex-protected listener map
   - Update() returns error on bind failure; closes replaced listeners synchronously before rebinding to avoid port conflicts, then binds listeners via startTCP/startUDP; returns early on the first bind failure (readyz becomes false, so the Pod receives no traffic and remaining listeners are unnecessary)
   - Exposes Ready() for readyz probe; becomes ready after Update() completes without bind failures
   - Bind failures are logged by ProxyRunner; the controller checks Ready() and sets Gateway Programmed=False with AddressNotUsable reason

7. **TCP/UDP Proxying** (`internal/proxy/tcp.go`, `internal/proxy/udp.go`, `internal/proxy/proxyprotocol.go`)
   - TCP: Accepts connections and proxies bidirectionally to backend services using io.Copy
   - TCP: Optionally prepends PROXY protocol header (`--proxy-protocol-version 1` for v1 text, `--proxy-protocol-version 2` for v2 binary) so backends can identify the original client IP
   - UDP: Forwards datagrams to backend services with session tracking and idle timeout
   - UDP: Optionally prepends PROXY protocol v2 binary header per datagram (`--proxy-protocol-version 2`); v1 is not supported for UDP

Key design decisions:
- Uses controller-runtime for Kubernetes integration
- Watches Gateway API resources including ListenerSet (GEP-1713) for decentralized listener ownership
- Single LoadBalancer Service per Gateway, with ports dynamically managed
- Owner references ensure Service cleanup when Gateway is deleted
- Proxy listeners run on all Pods via ProxyRunner (NeedLeaderElection=false), while Gateway/Service status updates are managed by the leader-elected controller
- MaxConcurrentReconciles set to 1 to ensure serial reconciliation of Gateway status updates
- Port conflict detection considers both port number and protocol (TCP:53 and UDP:53 can coexist)
- Dual-layer port conflict protection: webhook rejects at admission, controller detects at reconciliation
- Readyz probe gates on proxy listener bind completion via ProxyRunner, preventing traffic before listeners are ready
