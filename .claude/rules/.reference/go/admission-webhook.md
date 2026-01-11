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

// In Handle():
if h.enableSidecarContainers {
    sidecarContainer.RestartPolicy = func(p corev1.ContainerRestartPolicy) *corev1.ContainerRestartPolicy { return &p }(corev1.ContainerRestartPolicyAlways)
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

## Required Import

```go
"k8s.io/utils/ptr"
```

## Example

Copy from: `cluster/applications/exactly-one-pod-hook/pkg/webhook/webhook.go`
