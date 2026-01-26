# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The istio-addons directory contains Kubernetes manifests for configuring Istio service mesh addons. This component specifically manages the Vertical Pod Autoscaler (VPA) configuration for the Istio control plane (istiod).

## Common Development Commands

### Kustomize Commands
- `kubectl kustomize base/` - Preview the base configuration
- `kubectl kustomize overlays/dev/` - Preview the dev environment configuration with patches applied
- `kubectl apply -k overlays/dev/` - Apply the dev environment configuration to the cluster

### Validation Commands
- `kubectl apply --dry-run=client -f base/vertical_pod_autoscaler.yaml` - Validate the VPA manifest syntax
- `kubectl apply -k overlays/dev/ --dry-run=server` - Validate against the Kubernetes API server

## High-Level Architecture

### Structure
This follows the standard Kustomize directory pattern used throughout the Hippocampus cluster manifests:

- **`base/`** - Contains the core VPA configuration for istiod
  - `kustomization.yaml` - Declares the resources
  - `vertical_pod_autoscaler.yaml` - VPA configuration with Auto update mode

- **`overlays/dev/`** - Environment-specific customizations
  - `kustomization.yaml` - Applies namespace and patches
  - `patches/vertical_pod_autoscaler.yaml` - Adds resource policies with specific CPU/memory limits

### Key Configuration Details

The VPA is configured to:
- Target the `istiod` Deployment in the `istio-system` namespace
- Use `Auto` update mode in base configuration
- Apply resource policies in dev overlay:
  - Container: `discovery`
  - Min resources: 10m CPU, 16Mi memory
  - Max resources: 2000m CPU, 1Gi memory
  - Control mode: `RequestsOnly` (only adjusts requests, not limits)

### Integration Context

This component is part of the larger Istio installation and follows the manifest structure patterns defined in `/opt/hippocampus/cluster/manifests/README.md`. It's referenced by the ArgoCD application manifest at `argocd-applications/base/istio-addons.yaml`.