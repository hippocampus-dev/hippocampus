# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying cAdvisor (Container Advisor) as an additional monitoring component in the Hippocampus cluster. cAdvisor provides container resource usage and performance metrics.

## Common Development Commands

### Kubernetes Deployment
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl apply -k base/` - Deploy base configuration (without environment-specific patches)
- `kubectl -n kube-system get daemonset additional-cadvisor` - Check deployment status
- `kubectl -n kube-system logs -l name=additional-cadvisor` - View logs from all cAdvisor pods

### Kustomize Commands
- `kubectl kustomize overlays/dev/` - Preview the generated manifests for dev environment
- `kubectl kustomize base/` - Preview the base manifests
- `kustomize build overlays/dev/` - Build and output the complete manifest

## High-Level Architecture

### Component Structure
This is a Kustomize-based deployment with:
- **base/** - Core DaemonSet definition and base kustomization
- **overlays/dev/** - Development environment customizations (namespace, rolling update strategy)

### Key Design Decisions
1. **DaemonSet Deployment**: Ensures one cAdvisor instance per node labeled with `node-role.kubernetes.io/observed: ""`
2. **Security-First**: Runs as non-root, read-only filesystem, no capabilities, no service account token
3. **Selective Monitoring**: Only monitors Docker containers with specific whitelisted labels
4. **Network Metrics**: Explicitly enables TCP/UDP metrics collection

### Integration Points
- Deploys to `kube-system` namespace in dev environment
- Uses system-node-critical priority class for scheduling priority
- Mounts host filesystem paths (read-only) for container inspection
- Image sourced from Hippocampus mirror registry at `ghcr.io/kaidotio/hippocampus/mirror/`