# Webhook

Kubernetes admission webhook for mutating or validating resources.

## When to Use

- Mutating resources before creation/update
- Validating resources against policies
- Injecting sidecars, labels, or annotations

## Example

MUST copy from: `cluster/applications/exactly-one-pod-hook/`

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | deployment.yaml | Webhook server |
| manifests/ | service.yaml | Webhook endpoint |
| manifests/ | service_account.yaml | Pod identity |
| manifests/ | certificate.yaml | TLS certificate |
| manifests/ | issuer.yaml | cert-manager issuer |
| manifests/ | mutating_webhook_configuration.yaml | Webhook registration |
| manifests/ | pod_disruption_budget.yaml | Availability during updates |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/ | redis.yaml | Dependency service (if needed) |
| skaffold/ | etcd.yaml | Dependency service (if needed) |
| skaffold/patches/ | certificate.yaml | Development certificate |
| skaffold/patches/ | deployment.yaml | Development overrides |
| skaffold/patches/ | mutating_webhook_configuration.yaml | Development webhook config |

## Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/mutating_webhook_configuration.yaml`: Update webhook rules
- `skaffold/kustomization.yaml`: Set namespace, add `- path: patches/deployment.yaml` to patches
- `skaffold/namespace.yaml`: Set namespace name
- `skaffold/patches/deployment.yaml`: Update deployment name and container name to match the application
- `skaffold/patches/certificate.yaml`: Update certificate name and namespace
- `skaffold/patches/mutating_webhook_configuration.yaml`: Update webhook name and namespace
