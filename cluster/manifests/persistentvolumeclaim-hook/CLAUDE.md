# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kustomize manifests for the PersistentVolumeClaim Hook deployment. The PersistentVolumeClaim Hook is a Kubernetes mutating admission webhook that prevents storage size reduction for PersistentVolumeClaims and VolumeClaimTemplates in StatefulSets, preventing data loss by ensuring storage requests cannot be decreased.

## Common Development Commands

- `make dev` - Primary development command (runs from `/opt/hippocampus/cluster/applications/persistentvolumeclaim-hook/`)
- `kubectl apply -k overlays/dev/` - Apply development overlay to Kubernetes cluster
- `kubectl delete -k overlays/dev/` - Remove development deployment

## High-Level Architecture

### Directory Structure

This is a Kustomize-based manifest directory that references the main application in `/opt/hippocampus/cluster/applications/persistentvolumeclaim-hook/`:

- **base/** - Base Kustomization that points to the application manifests
- **overlays/dev/** - Development environment customizations including:
  - Namespace configuration
  - Network policies
  - Service mesh configuration (Istio sidecar, peer authentication, telemetry)
  - Certificate and webhook patches for local development
  - Pod disruption budget adjustments

### Overlay Patches

The dev overlay modifies the base deployment for local development:
- Sets namespace to `persistentvolumeclaim-hook`
- Configures cert-manager certificates with local DNS names
- Adjusts webhook configuration for local cluster access
- Reduces pod disruption budget for development
- Adds Istio service mesh integration

### Development Workflow

1. The actual webhook code is in `/opt/hippocampus/cluster/applications/persistentvolumeclaim-hook/`
2. This directory manages the Kubernetes deployment configuration
3. Changes to manifests should follow Kustomize patterns
4. Use `kubectl apply -k overlays/dev/` to deploy changes
5. The webhook prevents PVC storage reduction to protect against data loss

### Integration Notes

- Requires cert-manager for TLS certificate management
- Integrates with Istio service mesh when deployed with dev overlay
- Webhook runs on port 9443 with metrics on 8080
- Uses MutatingWebhookConfiguration to intercept PVC and StatefulSet updates