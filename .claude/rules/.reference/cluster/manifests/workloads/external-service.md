# External Service

HTTP services exposed via Istio Gateway.

## When to Use

- Public APIs, web applications, dashboards
- Services requiring custom domain and TLS termination

## Example

MUST copy from: `cluster/manifests/bakery/`

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| base/ | deployment.yaml | Pod template and replicas |
| base/ | service.yaml | ClusterIP service |
| base/ | horizontal_pod_autoscaler.yaml | CPU-based autoscaling |
| base/ | pod_disruption_budget.yaml | Availability during updates |
| overlays/dev/ | namespace.yaml | Namespace with pod-security labels |
| overlays/dev/ | network_policy.yaml | Ingress/egress rules |
| overlays/dev/ | peer_authentication.yaml | mTLS configuration |
| overlays/dev/ | sidecar.yaml | Istio egress rules |
| overlays/dev/ | telemetry.yaml | Tracing and metrics |
| overlays/dev/ | gateway.yaml | Istio Gateway for external access |
| overlays/dev/ | virtual_service.yaml | Routing rules |

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `deployment.yaml`: Update labels, container name, ports
- `service.yaml`: Update labels and ports
- `gateway.yaml`: Update host and TLS credential name
- `virtual_service.yaml`: Update host and destination
