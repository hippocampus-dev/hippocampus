# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repository contains Cilium addon configurations for the Hippocampus Kubernetes cluster. It provides additional networking policies and exposes the Hubble UI for network observability. The codebase uses Kustomize for environment-specific deployments.

## Common Development Commands

### Applying Configurations

```bash
# Apply dev environment configurations
kubectl apply -k overlays/dev/

# Apply base configurations only
kubectl apply -k base/

# Preview what will be applied (dry-run)
kubectl apply -k overlays/dev/ --dry-run=client -o yaml

# Delete configurations
kubectl delete -k overlays/dev/
```

### Validating Changes

```bash
# Validate YAML syntax
kubectl apply -k overlays/dev/ --dry-run=client

# Build and view the final manifests
kubectl kustomize overlays/dev/

# Check if resources were created
kubectl get ciliumclusterwidenetworkpolicy -A
kubectl get gateway,virtualservice -n kube-system
```

## High-Level Architecture

### Kustomize Structure
- **base/** - Core Cilium network policies that apply across all environments
  - Contains the L7 visibility policy for network traffic monitoring
- **overlays/** - Environment-specific configurations
  - **dev/** - Development environment with Istio ingress for Hubble UI

### Key Components

1. **CiliumClusterwideNetworkPolicy**: Enables Layer 7 visibility for pods labeled with `policy.cilium.io/l7-visibility: "true"`
2. **Istio Gateway**: Exposes Hubble UI through the ingress gateway
3. **Istio VirtualService**: Routes traffic to the Hubble UI service with retry logic

### Design Patterns

- **Label-based Policy Application**: Network policies are opt-in via pod labels
- **Environment Separation**: Base configurations are extended by environment-specific overlays
- **Istio Integration**: Uses Istio for ingress and traffic management
- **Namespace Isolation**: All resources deploy to `kube-system` namespace

### Adding New Environments

To add a new environment (e.g., staging or prod):

1. Create a new directory under `overlays/` (e.g., `overlays/staging/`)
2. Copy the kustomization.yaml structure from dev
3. Modify the Gateway hosts for your environment's domains
4. Update any environment-specific configurations

### Important Notes

- The L7 visibility policy allows all egress traffic but provides deep packet inspection
- DNS traffic (port 53) has special handling to ensure service discovery works
- The Hubble UI gateway supports both local development (nip.io) and cloud domains
- Retry policies in the VirtualService handle transient failures automatically