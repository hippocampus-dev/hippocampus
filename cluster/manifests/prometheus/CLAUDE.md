# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying Prometheus in agent mode as part of the Hippocampus monitoring stack. Prometheus runs as a lightweight metrics collector that forwards all data to Mimir for long-term storage.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy Prometheus to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests without applying
- `kubectl delete -k overlays/dev/` - Remove Prometheus deployment
- `kubectl rollout restart statefulset/prometheus -n prometheus` - Restart Prometheus pods

### Configuration Updates
- `kubectl create configmap prometheus-test --from-file=base/files/prometheus.yaml --dry-run=client -o yaml` - Test configuration syntax
- `cd base && kustomize edit set image prom/prometheus=prom/prometheus@sha256:NEW_DIGEST` - Update Prometheus image

### Debugging Commands
- `kubectl logs -n prometheus prometheus-0` - View Prometheus logs
- `kubectl exec -n prometheus prometheus-0 -- promtool check config /etc/prometheus/prometheus.yaml` - Validate configuration
- `kubectl port-forward -n prometheus svc/prometheus 9090:9090` - Access Prometheus UI locally

## High-Level Architecture

### Directory Structure
```
prometheus/
├── base/                    # Core Kustomize base resources
│   ├── files/              # Configuration files
│   │   └── prometheus.yaml # Main Prometheus configuration
│   ├── cluster_role.yaml   # RBAC permissions for service discovery
│   ├── stateful_set.yaml   # Prometheus StatefulSet definition
│   └── kustomization.yaml  # Base Kustomize configuration
└── overlays/
    └── dev/                # Development environment customizations
        ├── patches/        # Resource modifications for dev
        └── *.yaml          # Istio integration and network policies
```

### Key Design Patterns

1. **Agent Mode**: Prometheus runs with `--enable-feature=agent` flag, optimized for edge deployments with minimal local storage
2. **Remote Write Only**: All metrics are forwarded to Mimir at `http://mimir-distributor.mimir.svc.cluster.local:3100`
3. **Service Discovery**: Configured to automatically discover and scrape Kubernetes resources
4. **Istio Integration**: Development overlay includes service mesh configuration

### Configuration Details

**Scrape Targets**:
- Kubernetes API servers and control plane components
- Node metrics (kubelet, cAdvisor, resource metrics)
- Pod metrics via `prometheus.io/scrape: "true"` annotation
- kube-state-metrics (expected in kube-system namespace)

**Key Settings**:
- Global scrape interval: 30 seconds
- WAL retention: 5-30 minutes (configurable per job)
- Enabled features: expand-external-labels, exemplar-storage, extra-scrape-metrics
- Resource limits: 100m-2000m CPU, 2Gi-8Gi memory

**Security**:
- Non-root user (UID 65532)
- Read-only root filesystem
- All capabilities dropped
- RuntimeDefault seccomp profile

## Important Notes

- This deployment expects Mimir to be running in the `mimir` namespace
- kube-state-metrics must be deployed separately in `kube-system` namespace
- Uses mirrored container images from `ghcr.io/kaidotio/hippocampus/mirror/`
- Development environment includes 10Gi persistent volume for WAL storage
- Prometheus is exposed via Istio at `prometheus.minikube.127.0.0.1.nip.io` and `prometheus.kaidotio.dev` in dev