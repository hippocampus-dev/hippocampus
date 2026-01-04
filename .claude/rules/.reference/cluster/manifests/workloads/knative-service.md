# Knative Service

Serverless HTTP services that scale to zero.

## When to Use

- Services with variable traffic patterns
- Services that should scale to zero when idle
- HTTP webhooks with sporadic usage

## Example

MUST copy from: `cluster/applications/alerthandler/manifests/`

## Files

| File | Purpose |
|------|---------|
| service.yaml | Knative Service definition |
| kustomization.yaml | Image configuration |
| kustomizeconfig.yaml | Required for Kustomize to handle Knative |

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `service.yaml`: Update name, labels, container configuration
- `autoscaling.knative.dev/*` annotations: Adjust scaling parameters

## NetworkPolicy

Knative Services require ingress from three sources:

| Source | Namespace | Port | Purpose |
|--------|-----------|------|---------|
| cluster-local-gateway | istio-gateways | 8012 | Direct routing when Pod is running |
| activator | knative-serving | 8012 | scale-from-zero |
| autoscaler | knative-serving | 9090 | Metrics collection |

Example: `cluster/manifests/alerthandler/overlays/dev/network_policy.yaml`
