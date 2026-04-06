# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

pushgateway-statsd is a Kubernetes deployment of StatsD exporter that receives StatsD-formatted metrics and exports them in Prometheus format. Despite the "Pushgateway" name, this actually provides a StatsD exporter service.

## Common Development Commands

### Deployment
```bash
# Deploy via ArgoCD (automatic GitOps)
# Defined in: cluster/manifests/argocd-applications/base/pushgateway-statsd.yaml

# Manual deployment
kubectl apply -k overlays/dev/

# Generate manifests with Kustomize
kubectl kustomize overlays/dev/

# Dry-run deployment
kubectl apply -k overlays/dev/ --dry-run=client
```

### Testing and Validation
```bash
# Validate Kustomize configuration
kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -

# Check deployed resources
kubectl -n pushgateway-statsd get all
```

## High-Level Architecture

### Directory Structure
- **overlays/dev/**: Environment-specific configurations
  - Main kustomization.yaml with namespace prefix and labels
  - statsd-exporter/: StatSD exporter specific configurations
    - patches/: JSON patches for deployment, service, HPA, PDB
    - files/mapping.yaml: StatsD to Prometheus metric mapping rules
    - Istio configurations: mTLS, sidecar, telemetry

### Key Design Patterns
1. **Kustomize-based deployment**: Inherits from `cluster/manifests/utilities/statsd-exporter/` base
2. **Name prefixing**: All resources prefixed with `pushgateway-`
3. **Unified labeling**: `app.kubernetes.io/name: pushgateway` across all resources
4. **Istio integration**: Full service mesh support with mTLS, telemetry, and sidecar configuration
5. **High availability**: Pod disruption budgets and topology spread constraints
6. **Security**: Restricted Pod Security Standards compliance

### Service Architecture
- **StatsD Input**: UDP/TCP port 9125 for receiving StatsD metrics
- **Prometheus Export**: HTTP port 9102 for Prometheus scraping
- **Metric Mapping**: All metrics matched with `.*` regex, observer types treated as histograms
- **Label Preservation**: `honor_labels: true` maintains original metric labels

### Network Security
- Default deny NetworkPolicy with explicit ingress rules
- Allows metrics from any pod in the namespace
- Allows Prometheus scraping from monitoring namespace
- Istio PeerAuthentication for mTLS enforcement