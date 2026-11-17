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

Headless services (`clusterIP: None`) MUST define explicit `ports` when the namespace uses Istio sidecar injection.
Without port definitions, Istio does not register StatefulSet pod FQDNs (e.g., `{name}-0.{service}.{namespace}.svc.cluster.local`) in its service registry, causing `BlackHoleCluster` routing under `REGISTRY_ONLY` mode.

| Service has `ports` | Istio Registration | Pod FQDN Routing |
|---------------------|-------------------|------------------|
| Yes | Registered | Works |
| No | Not registered | BlackHoleCluster (connection reset) |

Note: Kubernetes DNS resolves pod FQDNs regardless of port definitions.
This requirement is Istio-specific.

## Peer Discovery Before Readiness

A headless service that members find each other through needs `publishNotReadyAddresses: true`.
Endpoints are otherwise withheld from a not-ready pod, and where readiness itself waits on discovery the two conditions wait on each other and neither arrives — a probe that reports quorum is one way in, an init container gating startup on the pod's own FQDN is another.

| Headless Service purpose | publishNotReadyAddresses |
|--------------------------|--------------------------|
| Member or instance discovery | `true` |
| Client traffic to individual pods | Omit |

Examples: `cluster/manifests/seata/base/service.yaml` (raft, init container waits on its own FQDN), `cluster/manifests/loki/base/service.yaml`, `cluster/manifests/mimir/base/service.yaml`, `cluster/manifests/tempo/base/service.yaml` and `cluster/manifests/pyroscope/base/service.yaml` (memberlist gossip ring, plus their `*-discovery` services for the query path)

## Readiness That Outruns Consensus

A health endpoint reporting only that the process started marks a replaced pod Ready before it has rejoined the group, so a rolling update takes the next one down while the cluster is still a member short.
Cover the gap with `minReadySeconds` on the StatefulSet, sized above the observed rejoin delay.

Raise `failureThreshold` alongside it, per `## Rolling Update Strategy` in `.claude/rules/cluster/manifests.md`.

Example: `cluster/manifests/seata/base/stateful_set.yaml` (`/health` reports the server, not the raft group, and Ready landed 7s ahead of the rejoin)

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `stateful_set.yaml`: Update labels, container name, ports, volumeMounts
- `service.yaml`: Update labels and ports
- `volumeClaimTemplates`: Adjust storage size
