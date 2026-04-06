# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes manifests for the Vertical Pod Autoscaler (VPA) deployment. VPA automatically adjusts CPU and memory requests/limits for containers based on usage patterns. The manifests follow a Kustomize-based structure with base configurations and environment-specific overlays.

## Directory Structure

- **`base/`** - Core VPA manifests including three main components:
  - `vpa-admission-controller` - Webhook that modifies pod specs with recommended resources
  - `vpa-recommender` - Analyzes resource usage and generates recommendations
  - `vpa-updater` - Evicts pods to apply new resource recommendations

- **`overlays/dev/`** - Development environment customizations:
  - Istio service mesh integration (sidecars, PeerAuthentication)
  - Prometheus metrics integration
  - Topology spread constraints for high availability
  - Leader election for multi-replica deployments

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy VPA to development environment
- `kubectl get vpa -A` - List all VPA objects across namespaces
- `kubectl describe vpa <name> -n <namespace>` - Check VPA recommendations
- `kubectl get pods -n vpa` - Check VPA component pods

### Debugging Commands
- `kubectl logs -n vpa deployment/vpa-recommender` - View recommender logs
- `kubectl logs -n vpa deployment/vpa-updater` - View updater logs
- `kubectl logs -n vpa deployment/vpa-admission-controller` - View admission controller logs

## Architecture Patterns

### Image Management
All container images are mirrored to `ghcr.io/hippocampus-dev/hippocampus/mirror/` for reliability and access control. Image digests are pinned in base/kustomization.yaml:
- cfssl/cfssl - Used for TLS certificate generation
- registry.k8s.io/autoscaling/vpa-* - VPA component images

### Security Configuration
- All containers run as non-root user (UID 65532)
- SecurityContext with minimal privileges (no capabilities, read-only root filesystem)
- TLS certificates generated at runtime using cfssl init container
- Network policies restrict traffic between components

### High Availability
- Multi-replica deployments for recommender and updater (2 replicas each)
- Leader election enabled to prevent conflicts
- Topology spread constraints across nodes and availability zones
- Pod disruption budgets to maintain availability during updates

### Prometheus Integration
The recommender is configured to use Prometheus/Mimir as the metrics backend:
- `--storage=prometheus`
- `--prometheus-address=http://mimir-proxy.mimir.svc.cluster.local:8080/prometheus`
- Queries cadvisor metrics for resource usage data
- 1-day history window with 1-hour resolution

### VPA Configuration
Key VPA parameters configured in the dev overlay:
- `--ignored-vpa-object-namespaces=kube-system,vpa` - Skip system namespaces
- `--oom-bump-up-ratio=1.2` - 20% increase on OOM
- `--oom-min-bump-up-bytes=134217728` - Minimum 128Mi increase on OOM
- `--min-replicas=2` - Don't update deployments with <2 replicas
- `--pod-update-threshold=0.1` - 10% change threshold before update
- `--evict-after-oom-threshold=10m` - Evict pods 10 minutes after OOM

## Development Workflow

1. **Modifying VPA behavior**: Edit args in `overlays/dev/patches/deployment.yaml`
2. **Updating images**: Modify digests in `base/kustomization.yaml`
3. **Testing changes**: Apply with `kubectl apply -k overlays/dev/`
4. **Creating VPA objects**: Use VerticalPodAutoscaler CRDs in target namespaces

## Important Notes

- VPA requires metrics-server or Prometheus for operation
- Admission controller webhook must be accessible by kube-apiserver
- VPA and HPA (Horizontal Pod Autoscaler) can conflict - use updateMode: "Off" for recommendation-only mode
- The admission controller generates self-signed certificates on startup
- All VPA components expose Prometheus metrics on port 894[2-4]