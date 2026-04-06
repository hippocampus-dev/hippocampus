# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Node Exporter Kubernetes manifests using Kustomize for deployment configuration. This is a Prometheus node exporter DaemonSet that runs on every node to collect system metrics.

## Common Development Commands

### Building Manifests
- `kustomize build base` - Build base configuration
- `kustomize build overlays/dev` - Build dev overlay with patches

### Validation
- `kustomize build overlays/dev | kubectl apply --dry-run=client -f -` - Validate manifest syntax

## High-Level Architecture

### Structure
- **`base/`** - Base Kubernetes resources
  - `daemon_set.yaml` - Node exporter DaemonSet configuration
  - `pod_disruption_budget.yaml` - PDB for high availability
  - `kustomization.yaml` - Base kustomization with image transformations

- **`overlays/dev/`** - Dev environment overlay
  - `kustomization.yaml` - Applies namespace and patches
  - `patches/` - Environment-specific modifications

### Key Configuration Patterns
1. **Security-hardened**: Runs as non-root user (65532) with minimal privileges
2. **Host access**: Mounts `/proc` and `/sys` as read-only for metrics collection
3. **High priority**: Uses `system-node-critical` priority class
4. **Tolerations**: Runs on all nodes including those with NoSchedule taints
5. **Image mirroring**: Images are mirrored to `ghcr.io/hippocampus-dev/hippocampus/mirror/`

### Deployment Workflow
1. Base manifests define core DaemonSet with security settings
2. Dev overlay adds namespace (`kube-system`) and environment patches
3. Kustomize handles image transformations and resource merging