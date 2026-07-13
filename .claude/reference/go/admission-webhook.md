# Admission Webhook Patterns

Patterns for Kubernetes admission webhooks that inject containers.

## Sidecar Container SecurityContext

When injecting sidecar containers via admission webhooks, always set SecurityContext with secure defaults:

```go
sidecarSecurityContext := &corev1.SecurityContext{
    AllowPrivilegeEscalation: ptr.To(false),
    Capabilities: &corev1.Capabilities{
        Drop: []corev1.Capability{"ALL"},
    },
    ReadOnlyRootFilesystem: ptr.To(true),
    RunAsUser:              ptr.To[int64](65532),
    RunAsNonRoot:           ptr.To(true),
    SeccompProfile: &corev1.SeccompProfile{
        Type: corev1.SeccompProfileTypeRuntimeDefault,
    },
}
```

| Field | Value | Purpose |
|-------|-------|---------|
| `AllowPrivilegeEscalation` | `false` | Prevent privilege escalation |
| `Capabilities.Drop` | `ALL` | Drop all Linux capabilities |
| `ReadOnlyRootFilesystem` | `true` | Prevent filesystem writes |
| `RunAsUser` | `65532` | Non-root user (matches distroless) |
| `RunAsNonRoot` | `true` | Enforce non-root |
| `SeccompProfile.Type` | `RuntimeDefault` | Enable default seccomp filtering |

## Native Sidecar Containers (Kubernetes 1.28+)

When injecting sidecar containers, support both native sidecars and legacy sidecars via `enableSidecarContainers` flag:

```go
type handler struct {
    // ...
    enableSidecarContainers bool
}

func p[T any](v T) *T {
    return &v
}

// In Handle():
if h.enableSidecarContainers {
    sidecarContainer.RestartPolicy = p(corev1.ContainerRestartPolicyAlways)
    pod.Spec.InitContainers = append(pod.Spec.InitContainers, sidecarContainer)
} else {
    pod.Spec.Containers = append(pod.Spec.Containers, sidecarContainer)
}
```

| Mode | Container Location | RestartPolicy | Kubernetes Version |
|------|-------------------|---------------|-------------------|
| Native sidecar (`true`) | `InitContainers` | `Always` | 1.28+ |
| Legacy sidecar (`false`) | `Containers` | (not set) | Any |

The flag should be configurable via CLI argument and environment variable:

```go
flag.BoolVar(&enableSidecarContainers, "enable-sidecar-containers", envOrDefaultValue("ENABLE_SIDECAR_CONTAINERS", false), "Enable native sidecar containers (requires Kubernetes 1.28+)")
```

### When NOT to Use Native Sidecars

Native sidecars are terminated AFTER main containers, not simultaneously. Do NOT use native sidecars when:

| Sidecar Requirement | Native Sidecar | Legacy Sidecar |
|---------------------|----------------|----------------|
| Must interact with main container on SIGTERM | No | Yes |
| Must capture data from main container before it exits | No | Yes |
| Must outlive main container (replication, cleanup) | Yes | No |
| No termination order dependency | Yes | Yes |

Example: `prometheus-metrics-proxy-hook` uses legacy sidecars because the sidecar must capture metrics from the main container when SIGTERM is received. Native sidecars would receive SIGTERM only after the main container has already terminated.

## Pointer Helper Selection

| Type | Pattern | Example |
|------|---------|---------|
| Simple types (`bool`, `int64`) | `ptr.To()` | `ptr.To(false)`, `ptr.To[int64](65532)` |
| Kubernetes constants | Local `p()` helper | `p(corev1.ContainerRestartPolicyAlways)` |

Use `ptr.To()` for primitive types where type inference works or explicit type parameter is simple. Use local `p[T any]()` helper for Kubernetes constants where `ptr.To` would require verbose type parameters like `ptr.To[corev1.ContainerRestartPolicy](...)`.

## Required Import

```go
"k8s.io/utils/ptr"
```

## Example

Copy from: `cluster/applications/exactly-one-pod-hook/pkg/webhook/webhook.go`
