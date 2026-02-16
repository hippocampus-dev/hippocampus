# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the prometheus-metrics-proxy-hook service, which is a Kubernetes admission webhook that injects a metrics proxy sidecar to ensure Prometheus scrapes complete before pod termination. The actual application source code is located at `/opt/hippocampus/cluster/applications/prometheus-metrics-proxy-hook/`.

## Common Development Commands

### Manifest Management
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests
- `kustomize build overlays/dev/` - Build manifests without kubectl

### Debugging and Monitoring
- `kubectl logs -n prometheus-metrics-proxy-hook -l app.kubernetes.io/name=prometheus-metrics-proxy-hook` - View webhook logs
- `kubectl port-forward -n prometheus-metrics-proxy-hook svc/prometheus-metrics-proxy-hook 8080:8080` - Access metrics endpoint

## High-Level Architecture

### Kustomize Structure
- **`base/`** - References the base manifests from the applications directory
- **`overlays/dev/`** - Development environment customizations including:
  - Network policies and Istio configurations
  - TLS certificate management via cert-manager
  - Prometheus metrics configuration

### Key Components

1. **Webhook Deployment**
   - 2 replicas for high availability
   - Mutating webhook intercepts pod creation
   - Injects metrics proxy sidecar as a regular container (not native sidecar)

2. **Security Configuration**
   - Istio sidecar injection enabled
   - PeerAuthentication for mTLS
   - NetworkPolicies for traffic control
   - Non-root containers with minimal privileges

### Configuration Flow

1. Pods annotated with `prometheus.io/wait: "true"` trigger the webhook
2. Webhook reads `prometheus.io/port` annotation for original metrics port
3. Webhook rewrites `prometheus.io/port` to `65532` (proxy port)
4. Sidecar container is injected to proxy metrics requests and track scrape times
5. On SIGTERM, sidecar captures and caches metrics, then waits for final scrape

### Important Notes

- The webhook requires TLS certificates managed by cert-manager
- Sidecar is injected as a regular container (not native sidecar) because it needs to receive SIGTERM simultaneously with the main container to capture metrics before the application terminates
- Metrics are exposed on port 8080 for Prometheus scraping
