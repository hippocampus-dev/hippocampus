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

Knative Services require NetworkPolicy rules for both infrastructure and caller traffic.

### Infrastructure Rules (Always Required)

| Source | Namespace | Port | Purpose |
|--------|-----------|------|---------|
| cluster-local-gateway | istio-gateways | 8012 | Direct routing when Pod is running |
| activator | knative-serving | 8012 | scale-from-zero |
| autoscaler | knative-serving | 9090 | Metrics collection |

### Mesh Caller Traffic

When a Knative Service is called from within the Istio mesh:

1. If pod is scaled to zero: mesh VirtualService routes to activator, then activator forwards to pod
2. If pod is running: mesh VirtualService routes directly to pod

This means:
- Activator needs to accept traffic from all mesh callers (handled in `cluster/manifests/knative-serving/overlays/dev/network_policy.yaml`)
- Individual Knative Service needs to allow specific callers when pod is running

### Activator NetworkPolicy (in knative-serving namespace)

Uses generic rule to allow all Istio mesh pods:

```yaml
- from:
    - namespaceSelector: {}
      podSelector:
        matchLabels:
          security.istio.io/tlsMode: istio
```

This covers all mesh callers without maintaining specific rules.

### Individual Service NetworkPolicy

Add specific ingress rules for known callers. Example for alerthandler (called by mimir-alertmanager):

```yaml
- from:
    - namespaceSelector:
        matchLabels:
          name: mimir
      podSelector:
        matchLabels:
          app.kubernetes.io/name: mimir
          app.kubernetes.io/component: alertmanager
  ports:
    - protocol: TCP
      port: 8012
```

Example for cloudevents-logger (called by kafka-broker-dispatcher):

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

### Examples

- alerthandler: `cluster/manifests/alerthandler/overlays/dev/network_policy.yaml`
- cloudevents-logger: `cluster/manifests/knative-eventing/overlays/dev/network_policy.yaml`
- activator: `cluster/manifests/knative-serving/overlays/dev/network_policy.yaml`

## Sidecar (Caller Configuration)

When calling a Knative Service from a pod with `REGISTRY_ONLY` Sidecar mode, add the ksvc namespace to egress hosts.

### Why Wildcard is Required

Knative creates per-Revision services (e.g., `alerthandler-00001`, `alerthandler-00002`). Use namespace wildcard:

```yaml
egress:
  - hosts:
      - alerthandler/*  # All services in alerthandler namespace
```

### When NOT Required

Callers with `ALLOW_ANY` mode don't need Sidecar configuration.

| Caller Mode | Sidecar Config |
|-------------|----------------|
| REGISTRY_ONLY | Add `{ksvc-namespace}/*` to egress hosts |
| ALLOW_ANY | Not required |

Note: Knative Eventing dispatchers use `ALLOW_ANY` by design. See [Knative Eventing](knative-eventing.md) for details.

### Example

mimir-alertmanager calling alerthandler:

```yaml
apiVersion: networking.istio.io/v1
kind: Sidecar
metadata:
  name: mimir-alertmanager
spec:
  workloadSelector:
    labels:
      app.kubernetes.io/name: mimir
      app.kubernetes.io/component: alertmanager
  outboundTrafficPolicy:
    mode: REGISTRY_ONLY
  egress:
    - hosts:
        - alerthandler/*  # Knative Service namespace
```

Note: `cluster-local-gateway` is NOT required for mesh-to-mesh traffic. The mesh VirtualService routes directly to activator or pod.

See: `cluster/manifests/mimir/overlays/dev/sidecar.yaml`
