# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kubernetes manifests for the url-shortener service deployment. Uses Kustomize for environment-specific configurations with Istio service mesh integration in the development environment.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k base/` - Deploy base configuration
- `kubectl apply -k overlays/dev/` - Deploy development environment with Istio integration
- `kustomize build base/` - Preview base manifests
- `kustomize build overlays/dev/` - Preview development manifests with patches

### Verification Commands
- `kubectl get all -n url-shortener` - Check deployed resources
- `kubectl describe deployment url-shortener -n url-shortener` - Inspect deployment details
- `kubectl logs -n url-shortener -l app=url-shortener` - View application logs

## High-Level Architecture

### Manifest Structure
The deployment uses Kustomize for configuration management:

**Base Layer (`base/`):**
- Core Kubernetes resources without environment-specific settings
- Includes Deployment, Service, HPA, and PodDisruptionBudget
- Image digest pinning for reproducible deployments

**Development Overlay (`overlays/dev/`):**
- Istio service mesh integration (sidecar injection, gateway, virtual service)
- Security policies (authorization, network policies, mTLS)
- Environment-specific patches for resource limits and configuration

### Key Deployment Patterns

**Resource Configuration:**
- Horizontal Pod Autoscaler scales between 1-3 replicas based on CPU/memory
- PodDisruptionBudget ensures at least 1 replica during voluntary disruptions
- Anti-affinity rules spread pods across zones and nodes for high availability

**Istio Integration (dev environment):**
- Automatic sidecar injection for service mesh features
- Gateway exposes service at `url-shortener.dev.127.0.0.1.nip.io`
- mTLS enforced via PeerAuthentication
- Authorization policies restrict access to specific service accounts

**Security Configuration:**
- Non-root container execution (UID 65532)
- Read-only root filesystem with explicit volume mounts
- Network policies control ingress/egress traffic
- Service runs on port 8080 internally

### Environment-Specific Patches

Development overlay applies patches to:
- Set http-kvs URL to development instance
- Configure resource requests/limits
- Add topologySpreadConstraints for pod distribution
- Adjust HPA thresholds for development workloads