# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes Descheduler configuration that optimizes pod placement by evicting pods from nodes based on defined policies. The descheduler runs as a CronJob every 10 minutes to rebalance workloads across the cluster. It uses Kustomize for manifest generation and is deployed via ArgoCD as part of the Hippocampus platform.

## Common Development Commands

### Building Manifests
```bash
# Build the dev overlay to see the generated manifests
kubectl kustomize overlays/dev

# Preview what will be deployed
kubectl kustomize overlays/dev | less

# Build and apply to dev environment
kubectl apply -k overlays/dev
```

### Validation
```bash
# Validate the generated manifests without applying
kubectl kustomize overlays/dev | kubectl apply --dry-run=client -f -

# Check current descheduler status
kubectl -n kube-system get cronjob descheduler
kubectl -n kube-system get jobs -l app=descheduler
```

### Testing Policy Changes
```bash
# Manually trigger a descheduler run
kubectl -n kube-system create job descheduler-manual --from=cronjob/descheduler

# View descheduler logs
kubectl -n kube-system logs -l app=descheduler --tail=100
```

## Architecture

### Directory Structure
- **`base/`** - Core descheduler configuration
  - `cron_job.yaml` - CronJob definition for periodic execution
  - `kustomization.yaml` - References upstream RBAC and configures image
  
- **`overlays/dev/`** - Development environment customization
  - `files/policy.yaml` - Descheduler policy configuration
  - `patches/cron_job.yaml` - Environment-specific patches
  - `kustomization.yaml` - Applies overlays and configmaps

### Policy Configuration
The descheduler implements three strategies:

1. **RemovePodsViolatingTopologySpreadConstraint** - Evicts pods violating topology spread constraints
   - Excludes StatefulSets and Jobs via label selectors
   - Considers both hard and soft constraints

2. **LowNodeUtilization** - Rebalances pods from overutilized to underutilized nodes
   - Low utilization threshold: CPU < 30%, Memory < 50%
   - Target threshold: CPU 60%, Memory 80%
   - Uses metrics server for real-time utilization

3. **RemoveFailedPods** - Cleans up pods in Evicted state

### Security Configuration
- Runs as non-root user (UID: 65532)
- Read-only root filesystem
- All capabilities dropped
- Seccomp profile applied
- Istio sidecar injection enabled

### Deployment Model
- Deployed to `kube-system` namespace
- Uses mirror registry: `ghcr.io/hippocampus-dev/hippocampus/mirror/`
- Managed by ArgoCD - changes should be committed, not applied directly
- CronJob schedule: `*/10 * * * *` (every 10 minutes)

## Development Workflow

1. **Modify Policy**: Edit `overlays/dev/files/policy.yaml` to adjust descheduling strategies
2. **Update Schedule**: Modify `overlays/dev/patches/cron_job.yaml` to change execution frequency
3. **Test Locally**: Use `kubectl kustomize` to preview changes
4. **Validate**: Run dry-run to ensure manifests are valid
5. **Commit**: Push changes to Git for ArgoCD deployment

### Adding New Strategies
To add a new descheduler strategy:
1. Add the plugin configuration under `pluginConfig` in policy.yaml
2. Enable it under `plugins.deschedule.enabled` or `plugins.balance.enabled`
3. Test with manual job execution before committing

### Monitoring Impact
```bash
# Watch pod evictions
kubectl get events --all-namespaces --field-selector reason=Evicted -w

# Check node utilization before/after
kubectl top nodes
```