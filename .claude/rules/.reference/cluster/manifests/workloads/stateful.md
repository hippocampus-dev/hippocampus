# Stateful Workload

Services requiring stable network identities and persistent storage.

## When to Use

- Databases (etcd, Redis, PostgreSQL)
- Distributed systems requiring stable pod identities
- Applications needing persistent volumes per replica

## Example

MUST copy from: `cluster/manifests/vault/`

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| base/ | stateful_set.yaml | Pod template with volumeClaimTemplates |
| base/ | service.yaml | Headless service (clusterIP: None) |
| base/ | pod_disruption_budget.yaml | Availability during updates |

## Headless Service Port Definition

Headless services (`clusterIP: None`) MUST define explicit `ports` when the namespace uses Istio sidecar injection. Without port definitions, Istio does not register StatefulSet pod FQDNs (e.g., `{name}-0.{service}.{namespace}.svc.cluster.local`) in its service registry, causing `BlackHoleCluster` routing under `REGISTRY_ONLY` mode.

| Service has `ports` | Istio Registration | Pod FQDN Routing |
|---------------------|-------------------|------------------|
| Yes | Registered | Works |
| No | Not registered | BlackHoleCluster (connection reset) |

Note: Kubernetes DNS resolves pod FQDNs regardless of port definitions. This requirement is Istio-specific.

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `stateful_set.yaml`: Update labels, container name, ports, volumeMounts
- `service.yaml`: Update labels and ports
- `volumeClaimTemplates`: Adjust storage size
