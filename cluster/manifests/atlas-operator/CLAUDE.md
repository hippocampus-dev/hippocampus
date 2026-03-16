# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes manifests for the Atlas Operator, which manages database schema migrations and versioning using Atlas (https://atlasgo.io). The operator watches for `AtlasSchema` and `AtlasMigration` custom resources and applies schema changes to target databases.

## Directory Structure

```
atlas-operator/
├── base/                              # Base Kustomize resources
│   ├── custom_resource_definition.yaml  # CRDs for AtlasSchema and AtlasMigration
│   ├── deployment.yaml                  # Operator controller deployment
│   ├── cluster_role.yaml                # Cluster-level RBAC permissions
│   ├── cluster_role_binding.yaml
│   ├── role.yaml                        # Namespace-scoped RBAC
│   ├── role_binding.yaml
│   ├── service_account.yaml
│   ├── pod_disruption_budget.yaml
│   └── kustomization.yaml
└── overlays/
    └── dev/                           # Dev environment overlay
        ├── kustomization.yaml
        ├── namespace.yaml
        ├── network_policy.yaml       # Default-deny + Prometheus scraping
        ├── peer_authentication.yaml  # Istio mTLS (STRICT mode)
        ├── sidecar.yaml              # Istio egress configuration
        ├── telemetry.yaml            # OpenTelemetry tracing + metrics
        └── patches/
            ├── deployment.yaml       # 2 replicas, topology spread, Istio sidecar
            └── pod_disruption_budget.yaml
```

## Key Configuration

### Base Configuration
- **Image**: `arigaio/atlas-operator` (mirrored to `ghcr.io/hippocampus-dev/hippocampus/mirror/arigaio/atlas-operator`)
- **Security**: Non-root user (1000), read-only root filesystem, no privilege escalation
- **Health checks**: Readiness probe on `/readyz:8081`, liveness probe on `/healthz:8081`
- **Metrics**: Exposed on port 8080

### Dev Overlay Specifics
- **Replicas**: 2 (with rolling update strategy)
- **Tag**: v0.7.11
- **Istio Integration**:
  - Sidecar injection enabled
  - Strict mTLS via PeerAuthentication
  - Egress limited to istiod, otel-agent, kubernetes API, and etcd
  - ALLOW_ANY outbound traffic policy for database connections
- **Observability**:
  - 100% trace sampling to OpenTelemetry agent
  - Prometheus metrics scraping every 15s
  - Envoy access logging enabled
- **Topology**: Spread across nodes and zones using `topologySpreadConstraints`

## Custom Resources Managed

The operator manages two primary CRDs:

1. **AtlasSchema** (`atlasschemas.db.atlasgo.io`)
   - Defines desired database schema state
   - Supports Atlas Cloud integration via token references
   - Status shows Ready condition

2. **AtlasMigration** (`atlasmigrations.db.atlasgo.io`)
   - Manages database migration execution
   - Reconciled by operator pods

## RBAC Permissions

The operator requires:
- **Cluster-level**: Read/write access to AtlasSchema and AtlasMigration CRDs
- **Namespace-level**:
  - Manage pods (for executing migrations)
  - Read secrets (for database credentials and Atlas tokens)
  - Read/write ConfigMaps
  - Create events
  - Manage deployments

## Applying Manifests

```bash
# Apply base manifests directly
kubectl apply -k base/

# Apply dev environment overlay
kubectl apply -k overlays/dev/

# View rendered manifests without applying
kubectl kustomize overlays/dev/

# Check operator logs
kubectl logs -n atlas-operator -l app.kubernetes.io/name=atlas-operator

# View CRD status
kubectl get atlasschemas -A
kubectl get atlasmigrations -A
```

## Modifying Configuration

### Adding New Environment Overlay
1. Copy `overlays/dev/` to `overlays/<env>/`
2. Update `kustomization.yaml` with environment-specific values
3. Modify patches as needed for replica count, resources, etc.

### Updating Operator Version
Edit `overlays/dev/kustomization.yaml`:
```yaml
images:
- name: arigaio/atlas-operator
  newTag: v0.x.x  # Update version here
```

### Network Policy Changes
The default network policy is deny-all. To allow traffic:
- Add ingress rules to `network_policy.yaml`
- Update Istio `sidecar.yaml` egress hosts for operator outbound connections

## Troubleshooting

### Operator Not Starting
1. Check pod status: `kubectl get pods -n atlas-operator`
2. View events: `kubectl describe deployment atlas-operator -n atlas-operator`
3. Check RBAC: Ensure ClusterRoleBinding and RoleBinding are created

### Schema Application Failing
1. Check AtlasSchema resource status: `kubectl describe atlasschema <name> -n <namespace>`
2. View operator logs for reconciliation errors
3. Verify database credentials in secrets are correct
4. Ensure operator can reach target database (check Sidecar egress rules)

### Metrics Not Scraped
- Verify NetworkPolicy allows Prometheus namespace access on port 15020
- Check Prometheus ServiceMonitor/PodMonitor configuration
- Ensure `prometheus.io/*` annotations are present on pods
