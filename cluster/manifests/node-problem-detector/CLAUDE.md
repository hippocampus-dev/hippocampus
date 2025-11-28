# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kustomize manifests for deploying node-problem-detector in Kubernetes. Node-problem-detector is a daemon that monitors node health and reports issues to the Kubernetes API.

## Common Development Commands

### Building Manifests
```bash
# Generate base manifests
kubectl kustomize base/

# Generate development environment manifests
kubectl kustomize overlays/dev/

# Apply to development cluster
kubectl apply -k overlays/dev/
```

### Validation
```bash
# Validate generated manifests
kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -

# Check current deployment status
kubectl get daemonset node-problem-detector -n kube-system
kubectl get pods -n kube-system -l app.kubernetes.io/name=node-problem-detector
```

## High-Level Architecture

### Directory Structure
- **`base/`** - Core configuration that's environment-agnostic
  - `daemon_set.yaml` - DaemonSet running on all nodes with privileged access
  - `pod_disruption_budget.yaml` - Ensures availability during updates
  - `files/kernel-monitor.json` - Problem detection patterns
  
- **`overlays/dev/`** - Development environment specific patches
  - Adds namespace: `kube-system`
  - Configures rolling update strategy
  - Adds Prometheus monitoring annotations

### Key Design Patterns

1. **Privileged DaemonSet**: Runs with privileged mode to access kernel logs and `/dev/kmsg`
2. **Security Context**: Despite privileged mode, follows security best practices:
   - Non-root user (65532)
   - Read-only root filesystem
   - Drops all capabilities except required ones
3. **Host Mounts**: Accesses critical host paths:
   - `/var/log` - System logs
   - `/dev/kmsg` - Kernel message buffer
   - `/etc/localtime` - Timezone synchronization
4. **Mirrored Images**: Uses `ghcr.io/kaidotio/hippocampus/mirror/` prefix for all images
5. **Prometheus Integration**: Exposes metrics on port 20257

### Monitored Problems

The kernel-monitor.json configures detection for:
- OOMKilling - Out of memory process kills
- TaskHung - Hung tasks
- UnregisterNetDevice - Network device issues
- KernelOops - Kernel panics
- NeighbourTableOverflow - ARP table overflow
- MemoryReadError - Memory hardware errors
- DockerHung - Docker daemon hangs
- ReadonlyFilesystem - Filesystem remounted read-only

### Important Notes

- This manifest follows the cluster-wide patterns documented in `/opt/hippocampus/cluster/manifests/README.md`
- Always validate manifests before applying to production
- PodDisruptionBudget ensures gradual rollout of updates
- ConfigMap generation is handled by Kustomize's configMapGenerator