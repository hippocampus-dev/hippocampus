# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes MutatingAdmissionPolicy implementation that provides declarative admission control using CEL (Common Expression Language). Unlike traditional webhook-based admission controllers, this uses the native policy-based approach introduced in Kubernetes 1.30+, requiring no running webhook servers or pods.

The current implementation automatically adds `NVIDIA_VISIBLE_DEVICES=none` environment variable to containers and initContainers that don't request GPU resources (`nvidia.com/gpu`), effectively hiding GPUs from non-GPU workloads.

## Common Development Commands

### Validation and Testing
```bash
# Validate Kubernetes manifests
kubectl apply --dry-run=client -f base/mutating_admission_policy.yaml
kubectl apply --dry-run=client -f base/mutating_admission_policy_binding.yaml

# Build with Kustomize
kustomize build base/
kustomize build overlays/dev/

# Apply to cluster (requires Kubernetes 1.30+ with MutatingAdmissionPolicy enabled)
kubectl apply -k base/
kubectl apply -k overlays/dev/

# Check policy status
kubectl get mutatingadmissionpolicies
kubectl get mutatingadmissionpolicybindings
kubectl describe mutatingadmissionpolicy pods.mutating.kaidotio.github.io
```

### Testing Mutations
```bash
# Create a test pod without GPU requests and verify environment variable
kubectl run test-pod --image=nginx --dry-run=server -o yaml | grep -A2 NVIDIA_VISIBLE_DEVICES

# Create a pod with GPU requests (should not be mutated)
kubectl run test-gpu-pod --image=nvidia/cuda:11.6.2-base-ubuntu20.04 \
  --limits='nvidia.com/gpu=1' --dry-run=server -o yaml
```

## High-Level Architecture

### MutatingAdmissionPolicy vs Traditional Webhooks

This directory demonstrates the modern approach to admission control:
- **No webhook server needed**: Policies run directly in the API server
- **CEL-based mutations**: Declarative expressions instead of Go code
- **Better performance**: No network calls to external webhooks
- **Simpler operations**: No certificates, deployments, or services to manage

### Policy Components

1. **MutatingAdmissionPolicy** (`mutating_admission_policy.yaml`):
   - Defines the mutation logic using CEL expressions
   - Targets Pod creation operations
   - Uses `ApplyConfiguration` patch type for strategic merge
   - `failurePolicy: Ignore` ensures pods can still be created if policy fails

2. **MutatingAdmissionPolicyBinding** (`mutating_admission_policy_binding.yaml`):
   - Links the policy to cluster resources
   - No namespace restrictions (applies cluster-wide)

### CEL Expression Pattern

The policy uses complex nested CEL expressions to:
1. Filter containers/initContainers without GPU resource limits
2. Map filtered containers to add the environment variable
3. Preserve existing container configuration while adding the env var

Key CEL patterns used:
- `?.` for safe navigation of optional fields
- `orValue()` for default values
- `filter()` and `map()` for list transformations
- `Object{}` syntax for constructing patch objects

### Limitations and Considerations

1. **CEL Complexity**: More complex mutations may require traditional webhooks
2. **Kubernetes Version**: Requires 1.30+ with feature gates enabled
3. **Debugging**: Limited compared to webhook logs and metrics
4. **Expression Size**: CEL expressions have size limits

### When to Use Traditional Webhooks

Consider the existing webhook patterns in this project (job-hook, litestream-hook, etc.) when:
- Complex business logic is required
- External API calls or database lookups are needed
- Detailed logging and metrics are important
- Supporting older Kubernetes versions

## Alternative Implementation Pattern

For complex mutations, follow the established webhook pattern in this project:
```
application-name/
├── main.go              # Webhook server implementation
├── Makefile            # Build and development commands
├── manifests/          # Kubernetes resources
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── certificate.yaml
│   └── mutating_webhook_configuration.yaml
└── skaffold.yaml       # Local development configuration
```

With development commands:
- `make dev` - Run with hot reload
- `skaffold dev --port-forward` - Deploy to local cluster