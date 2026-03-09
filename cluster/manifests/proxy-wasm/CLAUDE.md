# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying proxy-wasm filters to a Kubernetes cluster with Istio service mesh. It uses Kustomize to manage environment-specific configurations.

The manifests deploy WebAssembly filters that extend Envoy proxy capabilities, with the actual filter implementations located in `/opt/hippocampus/cluster/applications/proxy-wasm/`.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev` - Deploy to development environment
- `kubectl delete -k overlays/dev` - Remove deployment from development environment
- `kustomize build overlays/dev` - Preview generated manifests without applying

### Validation and Testing
- `kubectl kustomize overlays/dev | kubectl diff -f -` - Show differences between current and desired state
- `kubectl get all -n proxy-wasm` - View all resources in the proxy-wasm namespace
- `kubectl logs -n proxy-wasm -l app.kubernetes.io/name=proxy-wasm` - View application logs

## Architecture

### Kustomize Structure
- **base/** - References the core manifests from the applications directory
- **overlays/dev/** - Development environment customizations including:
  - Istio Gateway configuration for ingress
  - Namespace creation and configuration
  - Network policies for security
  - Service mesh configuration (sidecar, telemetry, virtual service)
  - Resource patches for deployment tuning

### Key Components
1. **Gateway** - Configures Istio ingress for proxy-wasm.minikube.127.0.0.1.nip.io and proxy-wasm.kaidotio.dev
2. **Sidecar** - Configures Envoy sidecar proxy behavior
3. **VirtualService** - Routes traffic through the Istio mesh
4. **NetworkPolicy** - Controls network access between pods
5. **PeerAuthentication** - Configures mTLS settings
6. **Telemetry** - Configures observability settings

### Deployment Patches
The overlay applies several patches to customize the base deployment:
- Resource limits (CPU: 5m request, Memory configured via Istio annotations)
- Topology spread constraints for high availability across nodes and zones
- Istio sidecar injection with specific resource allocations
- Rolling update strategy with controlled surge and unavailability