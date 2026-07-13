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

Gateway and VirtualService must always define at least these two hosts:

| Host Pattern | Purpose | Required |
|--------------|---------|----------|
| `{service}.minikube.127.0.0.1.nip.io` | Local development (no OAuth2 authentication) | Always |
| `{service}.kaidotio.dev` | Production (OAuth2 authentication via ext-authz) | Always |
| `{service}-public.kaidotio.dev` | Public access without ext-authz | When needed |

`{service}.kaidotio.dev` must always be added to `cluster/manifests/istio-gateways/overlays/dev/patches/authorization_policy.yaml` ext-authz hosts list.

### Domain Naming Convention

`-public` is an **additional** domain, not a replacement for the regular domain. All external services have `{service}.kaidotio.dev` with ext-authz. Services that also need unauthenticated access add `{service}-public.kaidotio.dev`:

| Condition | Hosts |
|-----------|-------|
| Default (ext-authz only) | `{service}.kaidotio.dev` |
| Needs public access (own auth, OAuth2 incompatible, etc.) | `{service}.kaidotio.dev` + `{service}-public.kaidotio.dev` |

When adding `-public`, also update Terraform:

| File | Change |
|------|--------|
| `terraform/cloudflare/access.tf` | Add `cloudflare_zero_trust_access_application` with bypass policy |
| `terraform/cloudflare/main.tf` | Add to `local.public_hosts` list |

Document the reason for `-public` in the service's manifests directory.

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `deployment.yaml`: Update labels, container name, ports
- `service.yaml`: Update labels and ports
- `gateway.yaml`: Update hosts (both minikube and kaidotio.dev)
- `virtual_service.yaml`: Update hosts and destination
- `istio-gateways/.../authorization_policy.yaml`: Add or exclude kaidotio.dev host (see Host Configuration)
