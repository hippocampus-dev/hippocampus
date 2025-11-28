# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Deployment Commands
```bash
# Deploy base configuration
kubectl apply -k base/

# Deploy development environment
kubectl apply -k overlays/dev/

# Verify deployment
kubectl -n kube-system get pods -l app.kubernetes.io/name=kube-state-metrics
kubectl -n kube-system get statefulsets,daemonsets -l app.kubernetes.io/name=kube-state-metrics
```

### Validation Commands
```bash
# Validate Kustomize build without applying
kubectl kustomize base/
kubectl kustomize overlays/dev/

# Check generated manifests
kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -
```

## High-Level Architecture

This directory contains Kubernetes manifests for kube-state-metrics deployment using Kustomize. The architecture implements a sophisticated dual-deployment strategy:

### Dual Deployment Strategy
1. **DaemonSet** - Runs on every node to collect Pod metrics locally
   - Monitors only Pods (`--resources=pods`)
   - Uses node affinity to collect metrics from local pods only (`--node=$(NODE_NAME)`)
   - Reduces cross-node network traffic for Pod metrics

2. **StatefulSet** - Runs as a sharded cluster for all other Kubernetes resources
   - Two shards (kube-state-metrics-a and kube-state-metrics-b) for load distribution
   - Each shard handles 50% of resources using consistent hashing
   - Monitors all Kubernetes resources except Pods

### Sharding Implementation
The StatefulSet uses sharding to handle large-scale clusters efficiently:
- `--shard=0 --total-shards=2` for kube-state-metrics-a
- `--shard=1 --total-shards=2` for kube-state-metrics-b
- Resources are distributed between shards using hash-based partitioning

### Security Configuration
All deployments follow strict security practices:
- Non-root user (UID: 65532)
- Read-only root filesystem
- All capabilities dropped
- Non-privileged containers
- Security context properly configured at both Pod and container levels

### Resource Organization
```
base/                       # Core resource definitions
├── kustomization.yaml      # Base Kustomize config with image definitions
├── cluster_role.yaml       # RBAC permissions for accessing Kubernetes resources
├── daemon_set.yaml         # Pod metrics collector (per-node)
├── stateful_set.yaml       # Sharded collectors for other resources
└── service.yaml           # Service exposing metrics endpoints

overlays/dev/              # Development environment customizations
├── kustomization.yaml     # Sets namespace and applies patches
└── patches/               # Environment-specific modifications
```

The manifests use the mirrored image `ghcr.io/kaidotio/hippocampus/mirror/registry.k8s.io/kube-state-metrics/kube-state-metrics` with digest `sha256:53d0bbbb108f4922e26aae60e292ac2278be14dc2e4bde368e67aa530c8472eb`.