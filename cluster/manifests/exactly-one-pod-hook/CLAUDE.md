# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kustomize manifests for deploying the `exactly-one-pod-hook` Kubernetes admission webhook. The webhook ensures only one pod of a given type runs at a time using distributed locking (Redis or etcd).

## Common Development Commands

### Primary Development
- `make dev` - Runs Skaffold with port-forwarding for local Kubernetes development (monitors changes and auto-redeploys)

### Kustomize Commands
- `kubectl kustomize overlays/dev` - Build and view the rendered manifests for dev environment
- `kubectl apply -k overlays/dev` - Apply the dev overlay to the cluster
- `kubectl delete -k overlays/dev` - Remove all resources from the dev overlay

## High-Level Architecture

### Directory Structure
- **`base/`** - Base Kustomization that references the application manifests
  - Points to `../../applications/exactly-one-pod-hook/manifests`
- **`overlays/dev/`** - Development environment overlay with:
  - `redis.yaml` - Redis StatefulSet (3 replicas) for distributed locking
  - `patches/` - Kubernetes patches for customizing base resources
  - Istio integration (sidecar, telemetry, peer authentication)
  - Network policies for security

### Key Components in Dev Overlay

1. **Redis Backend** - StatefulSet with 3 replicas for high availability
   - Uses ghcr.io/kaidotio/hippocampus/mirror/redis image
   - Persistent volume claims for data storage
   - Headless service for stable network identities

2. **Istio Integration**
   - `sidecar.yaml` - Configures Istio sidecar injection
   - `telemetry.yaml` - Metrics and tracing configuration
   - `peer_authentication.yaml` - mTLS settings for pod-to-pod communication

3. **Resource Patches**
   - `deployment.yaml` - Customizes webhook deployment settings
   - `service.yaml` - Service configuration adjustments
   - `mutating_webhook_configuration.yaml` - Webhook registration settings
   - `certificate.yaml` & `issuer.yaml` - TLS certificate management
   - `pod_disruption_budget.yaml` - Ensures webhook availability during updates

### Deployment Flow
1. Base manifests define core webhook resources
2. Dev overlay adds Redis backend and Istio integration
3. Patches customize resources for development environment
4. Skaffold manages the build-deploy cycle with hot reload

### Integration with Application
- Application code: `/opt/hippocampus/cluster/applications/exactly-one-pod-hook/`
- This manifest directory manages deployment configuration
- Skaffold bridges development workflow between code and manifests