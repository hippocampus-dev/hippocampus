# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes PriorityClass definitions for the Hippocampus cluster. PriorityClasses control pod scheduling priorities and preemption behavior.

## Common Development Commands

```bash
# Generate manifests for dev environment
kustomize build overlays/dev

# Apply to cluster (handled by ArgoCD in production)
kustomize build overlays/dev | kubectl apply -f -

# Validate manifests
kustomize build overlays/dev | kubectl apply --dry-run=client -f -
```

## Architecture

### Directory Structure
```
priorities/
├── base/
│   ├── kustomization.yaml
│   └── priority_class.yaml      # PriorityClass definitions
└── overlays/
    └── dev/
        └── kustomization.yaml   # Dev environment overlay
```

### PriorityClass Definitions

Six priority classes are defined, offering both preempting and non-preempting variants:

**Preempting Classes** (can evict lower priority pods):
- `low`: value -65535
- `medium`: value 0 (globalDefault: true)
- `high`: value 65535

**Non-Preempting Classes** (cannot evict other pods):
- `low-nonpreempting`: value -65535
- `medium-nonpreempting`: value 0
- `high-nonpreempting`: value 65535

### Key Configuration
- **Global Default**: `medium` class is the cluster-wide default
- **Sync Wave**: `-100` (deploys before most other resources)
- **Namespace**: Applied to `default` namespace in dev overlay

## Development Workflow

1. Modify PriorityClass definitions in `base/priority_class.yaml`
2. Test locally with `kustomize build`
3. Commit changes - ArgoCD will auto-sync to cluster
4. Monitor ArgoCD for sync status

## Usage Examples

To use a priority class in your pod specification:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-pod
spec:
  priorityClassName: high  # or high-nonpreempting for non-preempting behavior
  containers:
  - name: app
    image: myapp:latest
```

## Important Notes

- Use preempting classes (`low`, `medium`, `high`) for workloads that can evict less important pods
- Use non-preempting classes (`*-nonpreempting`) for workloads that should wait for resources
- The `medium` class is automatically assigned if no priorityClassName is specified
- Higher value means higher priority (range: -2147483648 to 1000000000)