# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kustomize-based Kubernetes manifests for deploying the token-request-server application. The manifests are organized using Kustomize overlays to support different environments.

## Common Development Commands

### Deployment
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl delete -k overlays/dev/` - Remove deployment from development environment
- `kustomize build overlays/dev/` - Preview generated manifests without applying

### Manifest Validation
- `kubectl apply -k overlays/dev/ --dry-run=client` - Validate manifests syntax
- `kubectl apply -k overlays/dev/ --dry-run=server` - Validate manifests against cluster API

## High-Level Architecture

### Directory Structure

- **base/** - Base Kustomization that references the application manifests
  - Points to `../../../applications/token-request-server/manifests`
  
- **overlays/dev/** - Development environment overlay
  - Adds Istio service mesh integration (AuthorizationPolicy, PeerAuthentication, Sidecar, VirtualService)
  - Configures networking (Gateway, NetworkPolicy)
  - Applies environment-specific patches for deployment, HPA, PDB, and service
  - Includes fluent-bit sidecar for audit log collection and forwarding to MinIO

### Key Manifest Components

1. **Istio Integration**:
   - `authorization_policy.yaml` - Controls access to the service
   - `peer_authentication.yaml` - Configures mTLS settings
   - `sidecar.yaml` - Sidecar proxy configuration
   - `virtual_service.yaml` - Traffic routing rules
   - `gateway.yaml` - Ingress gateway configuration

2. **Core Resources** (inherited from base):
   - Deployment with security best practices
   - HorizontalPodAutoscaler for scaling
   - PodDisruptionBudget for availability
   - Service and ServiceAccount
   - ClusterRoleBinding for Kubernetes API access

3. **Audit Logging**:
   - `files/fluent-bit.conf` - Fluent Bit configuration for collecting audit logs (loaded via configMapGenerator)
   - ConfigMap generated from `files/fluent-bit.conf` with immutable option enabled
   - Fluent Bit sidecar container configured to forward logs to dedicated MinIO instance
   - Audit logs are written to `/var/log/audit/audit.log` and automatically uploaded to `token-request-server-audit-logs` bucket
   - Shared volume (emptyDir) for audit log exchange between token-request-server and fluent-bit containers
   - Dedicated MinIO instance deployed in `minio/` subdirectory with 10Gi persistent storage

4. **Patches**:
   - Environment-specific modifications to base resources
   - Located in `overlays/dev/patches/`

### Development Workflow

When modifying manifests:
1. Base manifests should be edited in `/opt/hippocampus/cluster/applications/token-request-server/manifests/`
2. Environment-specific changes go in the appropriate overlay
3. Use Kustomize patches for overlay-specific modifications
4. Validate changes with `kustomize build` before applying

### Related Files

- Application source code: `/opt/hippocampus/cluster/applications/token-request-server/`
- Application CLAUDE.md with development commands: `/opt/hippocampus/cluster/applications/token-request-server/CLAUDE.md`
