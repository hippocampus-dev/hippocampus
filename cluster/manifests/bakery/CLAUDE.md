# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes manifests for the "bakery" service - an OAuth callback handler that manages cookie-based authentication. The manifests follow a Kustomize-based structure with security-hardened configurations and Istio service mesh integration.

## Common Development Commands

### Working with Manifests
- `kubectl kustomize base/` - Preview base configuration
- `kubectl kustomize overlays/dev/` - Preview dev overlay with all patches applied
- `kubectl apply -k overlays/dev/` - Deploy to dev environment
- `kubectl diff -k overlays/dev/` - Preview changes before applying

### Updating Image Digests
When updating the container image in base/kustomization.yaml:
1. Update the digest hash in the images section
2. Ensure the image name matches exactly what's in deployment.yaml

## High-Level Architecture

### Manifest Structure
The manifests use Kustomize overlays pattern:
- **base/** - Core resources (deployment, service, HPA, PDB)
- **overlays/dev/** - Environment-specific configurations with Istio integration

### Key Design Patterns

1. **Security-First Deployment**
   - Non-root user (65532), read-only filesystem, all capabilities dropped
   - RuntimeDefault seccomp profile
   - Resource limits enforced

2. **Istio Service Mesh Integration**
   - Gateway exposes service on `bakery.minikube.127.0.0.1.nip.io` and `bakery.kaidotio.dev`
   - Strict mTLS via PeerAuthentication
   - Sidecar egress limited to istiod and otel-agent
   - VirtualService with retry policies (3 attempts)

3. **Network Policies**
   - Default deny all traffic
   - Explicit allows for Prometheus metrics (port 15020) and Istio gateway ingress (port 8080)

4. **High Availability**
   - HPA: 1-5 replicas, 80% CPU target
   - Topology spread across zones and nodes
   - PDB: MaxUnavailable 1
   - Service traffic: PreferClose distribution

5. **Observability**
   - 100% distributed tracing to OpenTelemetry
   - Prometheus metrics every 15s
   - Access logging for all requests

### Important Configuration Notes

- The service runs on port 8080 with health checks at `/healthz`
- GOMAXPROCS and GOMEMLIMIT are automatically set from container limits
- Namespace uses Pod Security Standards with `restricted` enforcement
- Rolling updates: 25% surge, 1 max unavailable