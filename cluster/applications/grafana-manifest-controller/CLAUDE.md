# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

grafana-manifest-controller is a Kubernetes controller that manages Grafana dashboards as Kubernetes custom resources. It reconciles Dashboard resources by rendering Jsonnet to JSON and syncing the resulting dashboards to a Grafana instance via its API.

## Common Development Commands

### Development
- `make dev` - Creates a Kind cluster and runs Skaffold for hot-reload development
  - Creates cluster named `grafana-manifest-controller` using `kind.yaml` config
  - Runs `skaffold dev --port-forward` with automatic cleanup on exit

### Building
- `make all` - Run full test suite: gen, fmt, lint, tidy, test
- `make gen` - Generate CRDs from Go types using controller-gen
- `go build -trimpath -o grafana-manifest-controller main.go` - Build the Go binary locally

### Jsonnet Libraries
- `jb install` - Install Jsonnet dependencies (grafonnet, etc.)
- Libraries are bundled into the container at `/opt/jsonnet/vendor`
- `common.libsonnet` - Shared utilities (duplicated from `cluster/manifests/grafana/common.libsonnet`)

## Architecture

### Core Components

1. **Dashboard CRD** (`api/v1/dashboard_types.go`)
   - Defines the Dashboard custom resource
   - `grafanaServiceRef`: References the Grafana Service (ObjectReference)
   - Supports dashboard definition via:
     - `jsonnet`: Inline Jsonnet code
     - `configMapRef`: Reference to a ConfigMap containing Jsonnet
   - Optionally specifies Grafana `folder` for organization
   - Optionally sets `homeDashboard: true` to configure as organization home dashboard
   - Status tracks UID, URL, version, lastSyncedAt, and conditions

2. **Dashboard Controller** (`internal/controllers/dashboard_controller.go`)
   - Reconciles Dashboard resources
   - Resolves Grafana targets via `spec.grafanaServiceRef` and EndpointSlice, filtering ports by name `"http"`
   - Sets Host header to Service FQDN when accessing pod IPs directly (required for Istio REGISTRY_ONLY environments)
   - Syncs dashboards to all ready endpoints (supports multi-pod HA without headless Service)
   - Uses finalizers to clean up dashboards from all Grafana pods on deletion
   - Renders Jsonnet to JSON before syncing
   - Caches rendered Jsonnet output to avoid re-rendering when Dashboard generation and ConfigMap version are unchanged
   - Ensures target Grafana folder exists on each endpoint
   - Sets home dashboard via Grafana org preferences API when `homeDashboard: true`
   - Requeues every 1 minute for drift detection

3. **Jsonnet Renderer** (`internal/jsonnet/renderer.go`)
   - Wraps go-jsonnet VM with safe import restrictions
   - Only allows imports from configured library paths
   - Prevents path traversal outside allowed directories

4. **Grafana Client** (`internal/grafana/client.go`)
   - REST client for Grafana API operations
   - Accepts `host` parameter to set Host header on all requests (for Istio REGISTRY_ONLY compatibility when accessing pod IPs)
   - Upsert/delete dashboards
   - Folder management (create with deterministic UID, ensure via create-or-conflict)
   - Organization preferences (get/set home dashboard)

### Deployment Configuration

- Uses controller-runtime with leader election
- Metrics on port 8080, health probes on port 8081
- Jsonnet library path via `--jsonnet-library-path` or `JSONNET_LIBRARY_PATH`
- API group supports VARIANT prefix for development isolation

#### Grafana Instance Discovery

The controller discovers Grafana instances via `spec.grafanaServiceRef` on each Dashboard CR:

```yaml
spec:
  grafanaServiceRef:
    name: grafana
    namespace: grafana
```

1. References the Grafana Service by name and namespace
2. Resolves all ready pod IPs via EndpointSlice API, filtering by port name `"http"`
3. Computes Service FQDN and sets it as Host header when syncing to each pod IP (required for Istio REGISTRY_ONLY)

### Key Design Decisions

1. **Jsonnet-native**: Uses Jsonnet (with grafonnet library) for dashboard definitions
2. **ConfigMap support**: Dashboards can reference external ConfigMaps for separation of concerns
3. **Safe imports**: Jsonnet imports are restricted to prevent unauthorized file access
4. **Finalizer-based cleanup**: Ensures dashboards are removed from Grafana when CRs are deleted
5. **Drift detection**: Periodic requeue (every 1 minute) ensures dashboards stay in sync
6. **Change detection**: Before syncing, the controller compares normalized dashboard JSON (with `id` and `version` fields removed) to skip unnecessary API calls when content is unchanged, preventing version increment on every reconcile
7. **HA support**: `grafanaServiceRef` identifies the Grafana Service and resolves all pod IPs via EndpointSlice (filtered by port name `"http"`), syncing dashboards to every ready pod with Host header set to Service FQDN for Istio compatibility
8. **Home dashboard conflict detection**: When `homeDashboard: true` is set, the controller checks if another dashboard is already configured as home and fails reconciliation with `HomeDashboardConflict` condition to prevent unintended overwrites
9. **Jsonnet render caching**: Rendered Jsonnet output is cached per Dashboard resource, keyed by `Dashboard.Generation` and `ConfigMap.ResourceVersion`. Cache entries are evicted on resource deletion. This avoids expensive re-rendering (~100MB allocation per render) on every 1-minute reconcile cycle when neither the Dashboard spec nor the referenced ConfigMap has changed
