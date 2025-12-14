# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes manifests for the embedding-gateway service, which acts as a proxy for OpenAI API requests with S3-based embedding vector caching. The manifests follow the Kustomize pattern with base resources and environment-specific overlays.

## Development Commands

### Applying Manifests
```bash
# Apply to development environment
kubectl apply -k overlays/dev

# Apply base manifests only (not recommended for production)
kubectl apply -k base

# Preview changes without applying
kubectl kustomize overlays/dev

# Delete resources
kubectl delete -k overlays/dev
```

### Local Development with Skaffold
```bash
# From the application directory (/opt/hippocampus/cluster/applications/embedding-gateway)
skaffold dev --port-forward
```

## Architecture

### Manifest Structure
- **`base/`** - Core Kubernetes resources:
  - `deployment.yaml` - Main application deployment
  - `service.yaml` - Service exposure
  - `configmap.yaml` - Configuration data
  - `kustomization.yaml` - Base kustomization configuration

- **`overlays/dev/`** - Development environment customizations:
  - Environment-specific patches
  - Resource limits/requests adjustments
  - Development-specific configurations

### Key Configuration Points

1. **Service Configuration**:
   - Exposed on port 8000
   - Uses ClusterIP service type
   - Health checks on `/healthz` and readiness on `/readyz`

2. **ConfigMap Usage**:
   - S3 endpoint configuration for MinIO
   - OpenTelemetry configuration
   - Application-specific settings

3. **Environment Variables**:
   - `S3_ENDPOINT` - MinIO/S3 endpoint URL
   - `S3_BUCKET` - Bucket for embedding cache
   - `OTEL_*` - OpenTelemetry configuration
   - Secrets mounted from `minio-secret` for S3 credentials

### Integration Points

1. **MinIO/S3 Integration**:
   - Requires `minio-secret` with access/secret keys
   - Caches embedding vectors to reduce API calls

2. **OpenTelemetry**:
   - Traces exported to configured endpoint
   - Service name: `embedding-gateway`

3. **Health Monitoring**:
   - Liveness probe: `/healthz`
   - Readiness probe: `/readyz`
   - Initial delay: 30 seconds

## Common Tasks

### Updating Configuration
```bash
# Edit base configmap
$EDITOR base/configmap.yaml

# Apply changes
kubectl apply -k overlays/dev

# Verify deployment rolled out
kubectl rollout status deployment/embedding-gateway
```

### Debugging
```bash
# Check pod logs
kubectl logs -l app=embedding-gateway -f

# Describe deployment issues
kubectl describe deployment embedding-gateway

# Check service endpoints
kubectl get endpoints embedding-gateway
```

### Scaling
```bash
# Manual scaling
kubectl scale deployment embedding-gateway --replicas=3

# Or update in deployment.yaml and apply
```

## Important Notes

- The service depends on MinIO being available and properly configured
- S3 credentials must be present in the `minio-secret` before deployment
- Resource limits are environment-specific and defined in overlays
- The base manifests should remain environment-agnostic