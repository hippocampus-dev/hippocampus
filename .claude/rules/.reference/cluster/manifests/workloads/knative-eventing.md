# Knative Eventing

Event-driven services with Broker and Trigger for event routing.

## When to Use

- Event consumers (CloudEvents, Kafka)
- Pub/sub messaging patterns
- Services processing events from multiple sources

## Example

MUST copy from: `cluster/applications/cloudevents-logger/manifests/`

## Files

| File | Purpose |
|------|---------|
| service.yaml | Knative Service (event handler) |
| broker.yaml | Event broker |
| trigger.yaml | Event routing rules |
| kustomization.yaml | Image configuration |
| kustomizeconfig.yaml | Required for Kustomize to handle Knative |

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `service.yaml`: Update name, labels, container configuration
- `broker.yaml`: Update broker name
- `trigger.yaml`: Update trigger name, broker reference, filter attributes

## NetworkPolicy

Knative Services require ingress from three sources:

| Source | Namespace | Port | Purpose |
|--------|-----------|------|---------|
| cluster-local-gateway | istio-gateways | 8012 | Direct routing when Pod is running |
| activator | knative-serving | 8012 | scale-from-zero |
| autoscaler | knative-serving | 9090 | Metrics collection |

Example: `cluster/manifests/knative-eventing/overlays/dev/network_policy.yaml`
