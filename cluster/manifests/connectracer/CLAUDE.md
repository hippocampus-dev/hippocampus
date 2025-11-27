# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying ConnectRacer, an eBPF-based network connection tracer DaemonSet. The manifests follow Kustomize patterns with base configurations and environment-specific overlays (currently only `dev`).

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy to development environment using Kustomize
- `kubectl delete -k overlays/dev/` - Remove deployment from development environment
- `kubectl get daemonset connectracer -n connectracer` - Check DaemonSet status
- `kubectl logs -n connectracer -l app.kubernetes.io/name=connectracer` - View logs from all pods

### Validation and Testing
- `kubectl kustomize overlays/dev/` - Preview generated manifests without applying
- `kubectl diff -k overlays/dev/` - Show what would change if applied
- `kubectl rollout status daemonset/connectracer -n connectracer` - Monitor rollout status

## High-Level Architecture

### Directory Structure
```
connectracer/
├── base/                      # Base configurations
│   ├── daemon_set.yaml       # Core DaemonSet definition
│   ├── kustomization.yaml    # Base Kustomize configuration
│   └── pod_disruption_budget.yaml  # PDB for availability
└── overlays/
    └── dev/                  # Development environment
        ├── kustomization.yaml
        ├── namespace.yaml
        ├── network_policy.yaml
        ├── peer_authentication.yaml  # Istio mTLS config
        ├── sidecar.yaml             # Istio sidecar config
        ├── telemetry.yaml           # Istio telemetry config
        └── patches/
            ├── daemon_set.yaml      # Dev-specific patches
            └── pod_disruption_budget.yaml
```

### Key Design Patterns

1. **Kustomize-based Configuration**
   - Base manifests contain minimal, environment-agnostic configuration
   - Overlays add environment-specific settings (namespace, Istio integration, monitoring)
   - Patches modify base resources without duplication

2. **Security Configuration**
   - Runs with privileged context (required for eBPF)
   - Uses system-node-critical priority class
   - Configured with RuntimeDefault seccomp profile
   - Istio sidecar injection enabled in dev overlay

3. **Monitoring Integration**
   - Prometheus annotations for metrics scraping on port 8080
   - Istio telemetry configuration for distributed tracing
   - Container readiness probe on metrics port

4. **High Availability**
   - DaemonSet ensures one pod per node
   - PodDisruptionBudget limits disruptions during updates
   - Rolling update strategy with 10% max unavailable

### Manifest Conventions (following cluster standards)

- **Namespace**: Created in overlay with proper labels
- **NetworkPolicy**: Default-deny with exceptions for Envoy stats scraping
- **Istio Integration**: mTLS, sidecar resource limits, telemetry configuration
- **Resource Management**: CPU/memory limits set via Istio sidecar annotations
- **Monitoring**: Prometheus scrape annotations in overlay patches