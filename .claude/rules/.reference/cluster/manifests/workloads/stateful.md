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

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `stateful_set.yaml`: Update labels, container name, ports, volumeMounts
- `service.yaml`: Update labels and ports
- `volumeClaimTemplates`: Adjust storage size
