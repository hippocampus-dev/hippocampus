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

Knative Services used as event consumers require ingress from both infrastructure and event dispatchers.

### Infrastructure Rules (Always Required)

| Source | Namespace | Port | Purpose |
|--------|-----------|------|---------|
| cluster-local-gateway | istio-gateways | 8012 | Direct routing when Pod is running |
| activator | knative-serving | 8012 | scale-from-zero |
| autoscaler | knative-serving | 9090 | Metrics collection |

### Event Dispatcher Rules

When the Knative Service pod is running, the kafka-broker-dispatcher sends events directly to the pod (bypassing the activator). Add an ingress rule for the dispatcher:

```yaml
- from:
    - podSelector:
        matchLabels:
          app.kubernetes.io/name: knative-eventing
          app.kubernetes.io/component: kafka-broker-dispatcher
  ports:
    - protocol: TCP
      port: 8012
```

Example: `cluster/manifests/knative-eventing/overlays/dev/network_policy.yaml`

## Sidecar Configuration

### Dispatcher Components

Dispatchers (kafka-broker-dispatcher, kafka-channel-dispatcher, kafka-source-dispatcher) use `ALLOW_ANY` mode because they need to deliver events to arbitrary Knative Services in any namespace.

| Component | Sidecar Mode | Reason |
|-----------|--------------|--------|
| *-dispatcher | ALLOW_ANY | Delivers to arbitrary ksvc destinations |
| *-receiver | REGISTRY_ONLY | Receives from known sources only |
| *-controller | REGISTRY_ONLY | k8s API + internal communication |

### No Sidecar Config Needed for Dispatchers

Unlike callers with `REGISTRY_ONLY` mode, dispatchers don't need explicit egress hosts for Knative Service destinations.

See: `cluster/manifests/knative-eventing/overlays/dev/sidecar.yaml`

## SinkBinding

This cluster uses `SINK_BINDING_SELECTION_MODE=inclusion` on the eventing-webhook. In this mode, SinkBinding only injects environment variables into pods in namespaces that have the `bindings.knative.dev/include: "true"` label.

When creating a SinkBinding, add the label to the target namespace:

```yaml
metadata:
  labels:
    bindings.knative.dev/include: "true"
```

| Selection Mode | Behavior | Label Required |
|----------------|----------|----------------|
| `exclusion` (Knative default) | All namespaces affected unless labeled to exclude | No |
| `inclusion` (this cluster) | Only labeled namespaces affected | Yes |

Without this label, the SinkBinding's `K_SINK` environment variable will not be injected into the subject's pods.

Example: `cluster/manifests/memory-bank/overlays/dev/namespace.yaml`
