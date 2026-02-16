# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the grafana-manifest-controller service, which is a Kubernetes controller that manages Grafana dashboards as custom resources. The actual application source code is located at `/opt/hippocampus/cluster/applications/grafana-manifest-controller/`.

## Common Development Commands

### Manifest Management
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests
- `kustomize build overlays/dev/` - Build manifests without kubectl

### Debugging and Monitoring
- `kubectl logs -n grafana-manifest-controller -l app.kubernetes.io/name=grafana-manifest-controller` - View controller logs
- `kubectl port-forward -n grafana-manifest-controller svc/grafana-manifest-controller 8080:8080` - Access metrics endpoint

## High-Level Architecture

### Kustomize Structure
- **`base/`** - References the base manifests from the applications directory
- **`overlays/dev/`** - Development environment customizations including:
  - Network policies for secure communication
  - Istio configurations (Sidecar, PeerAuthentication, Telemetry)
  - Prometheus metrics configuration

### Key Components

1. **Controller Deployment**
   - Manages Dashboard custom resources
   - Syncs dashboards to Grafana via API
   - Bundled with Jsonnet libraries (grafonnet)
   - Metrics exposed on port 8080

2. **Security Configuration**
   - Istio sidecar injection enabled
   - PeerAuthentication for mTLS
   - NetworkPolicies for traffic control
   - Non-root container with minimal privileges

### Network Configuration

The controller requires egress to:
- Kubernetes API (`kubernetes.default.svc.cluster.local`)
- Grafana service (`grafana.grafana.svc.cluster.local`)
- OpenTelemetry agent (`otel-agent.otel.svc.cluster.local`)
- Istio control plane (`istiod.istio-system.svc.cluster.local`)

### Important Notes

- The controller uses leader election for high availability
- Dashboard definitions can be provided via Jsonnet inline or ConfigMap reference
- Metrics are exposed on port 8080 for Prometheus scraping
