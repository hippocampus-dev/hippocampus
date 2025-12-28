# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the assets storage service in the Hippocampus platform. It uses MinIO (S3-compatible object storage) to serve static assets like error pages and other web content. The service is deployed using Kustomize overlays with the dev environment configuration in `overlays/dev/`.

## Common Development Commands

### Working with Kustomize
- `kubectl kustomize overlays/dev/` - Build and view the final manifests
- `kubectl apply -k overlays/dev/` - Apply manifests to cluster
- `kubectl delete -k overlays/dev/` - Remove all resources

### Validating Changes
- `kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -` - Validate manifest syntax
- `kubectl kustomize overlays/dev/ | kubectl diff -f -` - Preview changes before applying

### Adding New Assets
1. Place new asset files in `overlays/dev/files/`
2. Update `overlays/dev/kustomization.yaml` to include them in the configMapGenerator
3. The assets-mc job will automatically upload them to MinIO on next deployment

## High-Level Architecture

### Service Components
- **MinIO StatefulSet**: S3-compatible storage backend configured with:
  - Single replica with 1Gi persistent storage
  - Exposed on port 9000 within cluster
  - Default credentials: minio/miniominio
  - Public bucket with anonymous read access

- **Assets Upload Job**: Kubernetes Job that:
  - Creates a public bucket in MinIO
  - Sets anonymous access permissions
  - Mirrors ConfigMap contents to MinIO storage
  - Runs with ArgoCD sync-wave -1 (after MinIO)

- **Istio Integration**: 
  - Sidecar injection enabled for all workloads
  - PeerAuthentication for mTLS
  - Telemetry configuration for metrics

### Directory Structure
```
assets/
└── overlays/
    └── dev/
        ├── files/              # Static asset files
        ├── minio/              # MinIO-specific overlays
        │   └── patches/        # Resource patches
        ├── job.yaml            # Asset upload job
        ├── kustomization.yaml  # Main kustomization
        └── *.yaml              # Namespace, network policies, Istio configs
```

### Key Design Patterns
- **Kustomize Overlays**: Base manifests from `/utilities/minio` with environment-specific patches
- **ConfigMap-based Assets**: Static files packaged as ConfigMaps, uploaded to object storage
- **ArgoCD Sync Waves**: Ordered deployment (MinIO at -2, Job at -1)
- **Security**: Restricted pod security, network policies, mTLS via Istio
- **Monitoring**: Prometheus metrics enabled via annotations