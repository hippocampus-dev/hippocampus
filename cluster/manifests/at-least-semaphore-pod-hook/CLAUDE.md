# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the at-least-semaphore-pod-hook service, which is a Kubernetes admission webhook that ensures a minimum number of pods can run concurrently using a distributed semaphore pattern. The actual application source code is located at `/opt/hippocampus/cluster/applications/at-least-semaphore-pod-hook/`.

## Common Development Commands

### Manifest Management
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests
- `kustomize build overlays/dev/` - Build manifests without kubectl

### Debugging and Monitoring
- `kubectl logs -n at-least-semaphore-pod-hook -l app.kubernetes.io/name=at-least-semaphore-pod-hook` - View webhook logs
- `kubectl logs -n at-least-semaphore-pod-hook -l app.kubernetes.io/component=redis` - View Redis logs
- `kubectl port-forward -n at-least-semaphore-pod-hook svc/at-least-semaphore-pod-hook 8080:8080` - Access metrics endpoint

## High-Level Architecture

### Kustomize Structure
- **`base/`** - References the base manifests from the applications directory
- **`overlays/dev/`** - Development environment customizations including:
  - Redis StatefulSet (3 replicas for Redlock algorithm)
  - Network policies and Istio configurations
  - TLS certificate management via cert-manager
  - Prometheus metrics configuration

### Key Components

1. **Webhook Deployment**
   - 2 replicas for high availability
   - Mutating webhook intercepts pod creation
   - Uses Redis for distributed locking
   - Injects sidecar containers for graceful shutdown

2. **Redis StatefulSet**
   - 3 replicas for Redlock consensus
   - Persistent storage for queue data
   - Headless service for direct pod access
   - PodDisruptionBudget ensures availability

3. **Security Configuration**
   - Istio sidecar injection enabled
   - PeerAuthentication for mTLS
   - NetworkPolicies for traffic control
   - Non-root containers with minimal privileges

### Configuration Flow

1. Pods annotated with `at-least-semaphore-pod-hook.kaidotio.github.io/at-least-semaphore-pod: "true"` trigger the webhook
2. Webhook checks Redis-backed semaphore queue
3. If semaphore allows, pod is admitted with injected sidecar
4. Sidecar handles cleanup on pod termination

### Important Notes

- The webhook requires TLS certificates managed by cert-manager
- Redis addresses are hardcoded in the deployment patch for the 3-replica StatefulSet
- Istio ServiceEntry is created for Redis pods to enable proper service mesh communication
- Metrics are exposed on port 8080 for Prometheus scraping