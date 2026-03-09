# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for [TiDB Operator](https://github.com/pingcap/tidb-operator), which is a Kubernetes operator that automates TiDB cluster deployment, scaling, and management. TiDB is a distributed SQL database that supports MySQL protocol and horizontal scaling.

## Structure

The manifests follow a standard Kustomize base/overlay pattern:

- **`base/`** - Base Kubernetes resources for TiDB Operator:
  - `custom_resource_definition.yaml` - CRDs for TiDB resources (TidbCluster, TidbMonitor, BackupSchedule, Restore, etc.)
  - `deployment.yaml` - TiDB Operator controller deployment
  - `cluster_role.yaml` / `cluster_role_binding.yaml` - RBAC for cluster-wide permissions
  - `role.yaml` / `role_binding.yaml` - Namespace-scoped RBAC
  - `service_account.yaml` - Service account for the operator
  - `pod_disruption_budget.yaml` - PDB for high availability
  - `kustomization.yaml` - Kustomize configuration with image registry mirrors

- **`overlays/dev/`** - Development environment overlay with:
  - `namespace.yaml` - Creates `tidb-operator` namespace with restricted pod security
  - `patches/deployment.yaml` - Adds HA configuration (2 replicas, topology spread, Istio sidecar)
  - `patches/pod_disruption_budget.yaml` - PDB adjustments
  - `peer_authentication.yaml` - Istio mTLS configuration (STRICT mode)
  - `telemetry.yaml` - Istio telemetry with 100% trace sampling, Prometheus metrics
  - `network_policy.yaml` - Network policies (default deny + Prometheus scraping)
  - `sidecar.yaml` - Istio sidecar egress configuration

## Common Operations

### Viewing Manifests

```bash
# Build and view base manifests
kubectl kustomize base/

# Build and view dev overlay
kubectl kustomize overlays/dev/
```

### Applying Changes

```bash
# Apply dev overlay to cluster
kubectl apply -k overlays/dev/

# Dry-run to preview changes
kubectl apply -k overlays/dev/ --dry-run=client
kubectl apply -k overlays/dev/ --dry-run=server
```

### Checking Operator Status

```bash
# Check operator deployment
kubectl -n tidb-operator get deployment tidb-operator

# Check operator logs
kubectl -n tidb-operator logs -l app.kubernetes.io/name=tidb-operator -f

# Check operator metrics
kubectl -n tidb-operator port-forward deployment/tidb-operator 6060:6060
# Then access http://localhost:6060/metrics
```

### Working with TiDB Custom Resources

```bash
# List TiDB clusters managed by this operator
kubectl get tidbcluster -A

# List TiDB monitors
kubectl get tidbmonitor -A

# List backup schedules
kubectl get backupschedules -A

# Get detailed info about a TiDB cluster
kubectl describe tidbcluster <name> -n <namespace>
```

## Key Architecture Details

### Image Registry Mirroring

All TiDB images are mirrored to `ghcr.io/hippocampus-dev/hippocampus/mirror/pingcap/*` to ensure availability and reduce external dependencies. The base kustomization specifies image digests for:
- `pingcap/tidb-operator` (v1.6.3)
- `pingcap/tidb-backup-manager` (v1.6.3)

### High Availability Configuration

The dev overlay configures the operator for HA:
- 2 replicas with RollingUpdate strategy (maxSurge: 25%, maxUnavailable: 1)
- Topology spread constraints across nodes and zones
- Pod disruption budget to prevent all replicas from being evicted

### Service Mesh Integration

Full Istio integration with:
- Sidecar injection enabled
- Strict mTLS for all traffic
- 100% distributed tracing sampling
- Envoy access logging for requests
- Egress controls limiting outbound traffic to specific services (istiod, otel-agent, kubernetes API, etcd)

### RBAC Permissions

The operator has extensive cluster-wide permissions to manage:
- Core Kubernetes resources (Pods, Services, PVCs, PVs, ConfigMaps, Secrets)
- Workload resources (StatefulSets, Deployments, Jobs, CronJobs)
- RBAC resources (Roles, RoleBindings within namespaces)
- PodDisruptionBudgets
- All TiDB custom resources (TidbCluster, TidbMonitor, Backup, Restore, etc.)
- Advanced StatefulSets (apps.pingcap.com API group)

### Network Security

Network policies implement defense-in-depth:
- Default deny all ingress/egress
- Allow Prometheus to scrape Envoy stats (port 15020)
- Namespace-level pod security enforcement (restricted)

## Custom Resources

The operator manages these TiDB-specific CRDs (all in `pingcap.com` API group):
- **TidbCluster** - Core TiDB database cluster definition
- **TidbMonitor** - Monitoring stack for TiDB
- **Backup** / **BackupSchedule** - Backup operations and schedules
- **Restore** - Database restore operations
- **TidbInitializer** - Initialize TiDB clusters with data
- **TidbDashboard** - TiDB dashboard deployment
- **TidbNGMonitoring** - Next-gen monitoring
- **DMCluster** - Data Migration cluster
- **TidbClusterAutoScaler** - Autoscaling configuration
- **CompactBackup** - Backup compaction

## Modifying Manifests

When making changes:

1. **Update base resources** if changes should apply to all environments
2. **Update overlay patches** for environment-specific changes
3. **Test with kustomize build**: `kubectl kustomize overlays/dev/`
4. **Validate CRDs** if modifying `custom_resource_definition.yaml` (file is large, ~2.6MB)
5. **Check image digests** in base kustomization when updating operator versions
6. **Update topology spread constraints** carefully - they affect scheduling decisions
7. **Review RBAC changes** - the operator requires broad permissions for TiDB management
