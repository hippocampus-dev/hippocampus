# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build and Deploy
- **Apply manifests locally**: `kubectl apply -k overlays/dev/`
- **Build kustomization**: `kubectl kustomize overlays/dev/`
- **Validate manifests**: `kubectl apply -k overlays/dev/ --dry-run=client`
- **Check current deployment**: `kubectl -n kube-system get deployment dns-autoscaler`
- **View autoscaler logs**: `kubectl -n kube-system logs -l app=dns-autoscaler`
- **Check CoreDNS scaling**: `kubectl -n kube-system get deployment coredns`

### ArgoCD Deployment
- This manifest is deployed via ArgoCD application defined in `/opt/hippocampus/cluster/manifests/argocd-applications/base/dns-autoscaler.yaml`
- **Sync application**: `argocd app sync dns-autoscaler`
- **Check sync status**: `argocd app get dns-autoscaler`

## High-Level Architecture

### Purpose
DNS autoscaler automatically scales CoreDNS replicas based on cluster size using the Kubernetes cluster-proportional-autoscaler. It monitors cluster metrics and adjusts CoreDNS deployment to maintain DNS performance as the cluster grows or shrinks.

### Structure
- **base/**: Contains base Kubernetes resources (deployment, kustomization, pod disruption budget)
- **overlays/dev/**: Development environment patches with specific scaling parameters and topology constraints

### Key Components
1. **Cluster Proportional Autoscaler**: The main controller that monitors cluster size and scales CoreDNS
2. **Scaling Algorithm**: Linear scaling based on:
   - 1 replica per node (nodesPerReplica: 1)
   - 1 replica per 16 CPU cores (coresPerReplica: 16)
   - Minimum 4 replicas, maximum 100 replicas

### Security Configuration
- Runs as non-root user (UID: 65532)
- Read-only root filesystem with memory-based temp volume
- All capabilities dropped
- Security context enforced at both pod and container level

### Important Notes
- The autoscaler targets `deployment/coredns` in the `kube-system` namespace
- Uses a mirror registry (`ghcr.io/kaidotio/hippocampus/mirror/`) for the container image
- Configured with topology spread constraints for high availability across hosts and zones
- Pod disruption budget allows maximum 1 unavailable pod during updates