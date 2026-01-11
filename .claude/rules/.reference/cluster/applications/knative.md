# Knative

Serverless services using Knative Serving or Eventing.

## When to Use

- Scale-to-zero HTTP services
- Event-driven services with Broker/Trigger

## Example

Copy from: `alerthandler`

## Files (cluster/applications/{app}/)

| Directory | File | Purpose |
|-----------|------|---------|
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | kustomizeconfig.yaml | Kustomize Knative support |
| manifests/ | service.yaml | Knative Service |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/patches/ | service.yaml | Development overrides |

## Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/service.yaml`: Update service name and container
- `skaffold/kustomization.yaml`: Set namespace (no labels block needed)
- `skaffold/namespace.yaml`: Set namespace name
