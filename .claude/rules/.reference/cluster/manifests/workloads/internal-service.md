# Internal Service

Cluster-internal HTTP/gRPC services without external ingress.

## When to Use

- Backend APIs called by other services
- Internal proxies, helper services
- Services not directly accessed by users

## Example

MUST copy from: `cluster/manifests/bakery/` (exclude gateway.yaml and virtual_service.yaml)

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| base/ | deployment.yaml | Pod template and replicas |
| base/ | service.yaml | ClusterIP service |
| base/ | horizontal_pod_autoscaler.yaml | CPU-based autoscaling |
| base/ | pod_disruption_budget.yaml | Availability during updates |
| overlays/dev/ | namespace.yaml | Namespace with pod-security labels |
| overlays/dev/ | network_policy.yaml | Allow specific caller namespaces |
| overlays/dev/ | peer_authentication.yaml | mTLS configuration |
| overlays/dev/ | sidecar.yaml | Istio egress rules |
| overlays/dev/ | telemetry.yaml | Tracing and metrics |

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `deployment.yaml`: Update labels, container name, ports
- `service.yaml`: Update labels and ports
- `network_policy.yaml`: Update allowed caller namespaces
