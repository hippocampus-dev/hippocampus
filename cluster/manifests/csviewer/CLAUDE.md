# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CSViewer is a Kubernetes manifest repository for deploying the CSViewer application. This directory contains the Kubernetes resource definitions using Kustomize for managing different environments.

## Common Development Commands

### Kustomize Commands
- `kubectl kustomize overlays/dev` - Build and view the development environment manifests
- `kubectl apply -k overlays/dev` - Apply the development environment to cluster
- `kubectl diff -k overlays/dev` - Show what would change before applying

### Validation Commands
- `kubectl --dry-run=client -f base/deployment.yaml` - Validate deployment syntax
- `kustomize build overlays/dev | kubectl apply --dry-run=server -f -` - Server-side validation

## High-Level Architecture

### Directory Structure
The manifests follow the Kustomize pattern:
- `base/` - Core Kubernetes resources shared across environments
  - `deployment.yaml` - Main application deployment
  - `service.yaml` - ClusterIP service definition
  - `horizontal_pod_autoscaler.yaml` - HPA for auto-scaling
  - `pod_disruption_budget.yaml` - PDB for availability
  - `kustomization.yaml` - Base kustomization configuration

- `overlays/dev/` - Development environment specific configurations
  - Istio service mesh integration (gateway, virtual service, sidecar)
  - Network policies for security
  - Environment-specific patches

### Key Design Patterns

1. **Resource Organization**: Base resources define the core application, overlays add environment-specific modifications
2. **Istio Integration**: Development overlay includes full Istio service mesh configuration for traffic management
3. **Security**: Network policies restrict traffic, non-root container execution (UID 65532)
4. **High Availability**: HPA scales 1-3 replicas based on CPU/memory, PDB ensures minimum availability

### Important Configuration Details

When modifying manifests:
- Maintain exact YAML structure and field ordering as existing files
- All services use port 8080 internally
- Container runs as user 65532 (nonroot)
- Resource limits: 50Mi memory, 50m CPU (requests), 100Mi/100m (limits)
- Istio virtual service handles both internal and external traffic
- Development uses gateway host: csviewer-dev.example.com