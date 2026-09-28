# Knative

Serverless services using Knative Serving or Eventing.

## Pattern Selection

| Condition | Pattern | Example |
|-----------|---------|---------|
| HTTP service (webhook, API, event sink) | Serving | `alerthandler` |
| Event consumer (Broker/Trigger) | Eventing | `cloudevents-logger` |

## Serving

Scale-to-zero HTTP services called directly or via webhook.

### Example

Copy from: `alerthandler`

### Files (cluster/applications/{app}/)

| Directory | File | Purpose |
|-----------|------|---------|
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | kustomizeconfig.yaml | Kustomize Knative support |
| manifests/ | service.yaml | Knative Service |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/patches/ | service.yaml | Development overrides |

### Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/service.yaml`: Update service name and container
- `skaffold/kustomization.yaml`: Set namespace (no labels block needed)
- `skaffold/namespace.yaml`: Set namespace name

## Eventing

Event-driven services consuming CloudEvents via Broker and Trigger.

### Example

Copy from: `cloudevents-logger`

### Files (cluster/applications/{app}/)

| Directory | File | Purpose |
|-----------|------|---------|
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | kustomizeconfig.yaml | Kustomize Knative support (Trigger name references) |
| manifests/ | service.yaml | Knative Service (event handler) |
| manifests/ | broker.yaml | Event broker |
| manifests/ | trigger.yaml | Event routing rules |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/ | ping_source.yaml | PingSource for local testing |
| skaffold/patches/ | service.yaml | Development overrides |

### Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/service.yaml`: Update name, labels, container configuration
- `manifests/broker.yaml`: Update broker name
- `manifests/trigger.yaml`: Update trigger name, broker reference, filter attributes
- `skaffold/kustomization.yaml`: Set namespace, add `ping_source.yaml` to resources
- `skaffold/ping_source.yaml`: Update sink broker name (must include `skaffold-` prefix from namePrefix)
