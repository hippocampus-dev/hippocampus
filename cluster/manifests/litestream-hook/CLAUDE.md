# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying the litestream-hook admission webhook using Kustomize. It provides environment-specific overlays for the base application manifests located in `/opt/hippocampus/cluster/applications/litestream-hook`.

## Common Development Commands

### Primary Development
- `make dev` - Run from the application directory (`../../applications/litestream-hook/`) for hot-reload development with Skaffold

### Deployment
- Apply manifests using Kustomize:
  ```bash
  kubectl apply -k overlays/dev
  ```

## High-Level Architecture

### Kustomize Structure
1. **Base Layer** (`base/`):
   - References the application manifests from `../../../applications/litestream-hook/manifests`
   - Provides foundation for all environments

2. **Dev Overlay** (`overlays/dev/`):
   - Adds environment-specific resources:
     - Namespace with pod security standards (`restricted` enforcement)
     - Network policies for traffic control
     - Istio integration (PeerAuthentication, Sidecar configuration)
     - Telemetry configuration
   - Patches base resources:
     - Deployment: Adds replicas (2), topology spread constraints, Istio sidecar injection
     - Service: Environment-specific modifications
     - Certificate/Issuer: TLS certificate management via cert-manager
     - MutatingWebhookConfiguration: Webhook registration patches
     - PodDisruptionBudget: Availability guarantees

### Integration Points
- **Istio Service Mesh**: Sidecar injection enabled with resource limits (CPU: 30m-1000m, Memory: 64Mi-1Gi)
- **Prometheus Monitoring**: Metrics exposed on port 8080 at `/metrics`
- **Pod Security**: Enforces `restricted` pod security standard at namespace level
- **High Availability**: 2 replicas with topology spread across nodes and zones

### Key Configuration Patterns
- Uses Kustomize patches to overlay environment-specific settings
- Maintains separation between application code and deployment configuration
- Leverages Kubernetes native features (topology spread, PDB) for reliability