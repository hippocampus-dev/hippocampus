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
| overlays/dev/patches/ | service.yaml | trafficDistribution: PreferClose |

## Host Configuration

Gateway and VirtualService must define both hosts together:

| Host Pattern | Purpose |
|--------------|---------|
| `{service}.minikube.127.0.0.1.nip.io` | Local development (no OAuth2 authentication) |
| `{service}.kaidotio.dev` | Production (OAuth2 authentication via ext-authz) |

When adding `{service}.kaidotio.dev`, determine whether to add it to `cluster/manifests/istio-gateways/overlays/dev/patches/authorization_policy.yaml` hosts list:

| Condition | Action |
|-----------|--------|
| Default | Add to ext-authz hosts list |
| System cannot support OAuth2 (e.g., browser-generated W3C Reporting API) | Exclude from ext-authz |
| Service has its own AuthorizationPolicy with fine-grained access control | Exclude from ext-authz |

Only exclude when OAuth2 authentication is technically impossible or when the service already implements its own AuthorizationPolicy. When excluding, document the reason in the service's manifests directory.

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `deployment.yaml`: Update labels, container name, ports
- `service.yaml`: Update labels and ports
- `gateway.yaml`: Update hosts (both minikube and kaidotio.dev)
- `virtual_service.yaml`: Update hosts and destination
- `istio-gateways/.../authorization_policy.yaml`: Add or exclude kaidotio.dev host (see Host Configuration)
