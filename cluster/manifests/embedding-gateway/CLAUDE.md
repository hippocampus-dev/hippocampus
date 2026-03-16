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
  - `ext-proc-proxy/` - Entry point proxy that computes x-body-hash header via ext-proc
  - `varnish/` - HTTP cache proxy layer (Varnish) for response caching
  - `endpoint-broadcaster/` - Bulk cache purge broadcaster for Varnish

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
   - Readiness probe: HTTP GET `/healthz`
   - Initial delay: 5 seconds

4. **Ext-Proc Proxy** (dev overlay):
   - Service: `embedding-gateway` on port 8080 (entry point for external traffic)
   - Purpose: Computes `x-body-hash` header from request body before forwarding to Varnish
   - Traffic flow: `istio-ingressgateway` -> `ext-proc-proxy` -> `varnish` -> `embedding-gateway-backend`
   - Runs ext-proc gRPC service as sidecar container (port 50051)
   - EnvoyFilter configures `SIDECAR_INBOUND` with two HTTP filters:
     - `envoy.filters.http.ext_proc`: Calls ext-proc with `request_body_mode: BUFFERED` to compute `x-body-hash`
     - `envoy.filters.http.lua`: Rewrites Host header to `embedding-gateway-varnish` (required because socat is L4 proxy and doesn't modify Host header)
   - DestinationRule `embedding-gateway-varnish` uses `consistentHash.httpHeaderName: x-body-hash` for sticky routing

5. **Varnish Cache Layer** (dev overlay):
   - Service: `embedding-gateway-varnish` on port 6081
   - Backend: `embedding-gateway-backend` on port 8080
   - Caches HTTP responses (1h TTL, 24h grace period)
   - Bypasses cache for `/healthz` and `/metrics` endpoints
   - POST request caching requires non-empty `X-Body-Hash` header (requests without it or with empty value bypass cache)
   - Host header rewrite: Varnish sets `Host: embedding-gateway-backend` in `vcl_backend_fetch` to ensure Envoy sidecar routes to the correct backend (original request Host header would cause routing failures)
   - Cache invalidation (unified PURGE method):
     - `PURGE /path`: Invalidate specific URL
     - `PURGE` with `X-Purge-All: true`: Clear all cache
     - `PURGE` with `Surrogate-Key: tag`: Invalidate by tag (requires backend to set `Surrogate-Key` header)
   - Access control (AuthorizationPolicy `varnish` with multiple rules):
     - Rule 1: Allows all methods except PURGE from `ext-proc-proxy`
     - Rule 2: Allows only PURGE from `endpoint-broadcaster`
     - Requests not matching any rule are implicitly denied
   - Access logging (EnvoyFilter `varnish-access-log`):
     - JSON format output to stdout
     - Includes `x-body-hash` header for cache key correlation

6. **Endpoint Broadcaster** (dev overlay):
   - Service: `embedding-gateway-endpoint-broadcaster` on port 8080
   - ServiceAccount: `embedding-gateway-endpoint-broadcaster` (used for AuthorizationPolicy principal matching)
   - Purpose: Broadcasts PURGE requests to all Varnish pods for bulk cache invalidation
   - Target: `embedding-gateway-varnish-headless` service (headless service for direct pod access)
   - Access control: Only PURGE method is allowed from `istio-ingressgateway`
   - External access:
     - `embedding-gateway-purge.minikube.127.0.0.1.nip.io` (local development, no authentication)
     - `embedding-gateway-purge.kaidotio.dev` (production, OAuth2 authentication via ext-authz)
   - Note: PURGE method is used instead of Varnish's BAN method because Envoy's HTTP parser only recognizes standard methods. PURGE starts with 'P' and is recognized by Envoy.

### ArgoCD Sync Order (dev overlay)

Resources are deployed in a specific order using `argocd.argoproj.io/sync-wave` annotations:

| Wave | Resource | Reason |
|------|----------|--------|
| -3 | `service_backend.yaml` | Service DNS must exist before Varnish starts |
| -2 | `minio/`, `varnish/`, `ext-proc-proxy/`, `endpoint-broadcaster/` | Infrastructure dependencies |
| -1 | `job.yaml`, `sidecar.yaml` | Post-infrastructure setup |
| 0 | Other resources | Default wave |

The backend Service must be deployed before Varnish to ensure DNS resolution succeeds during Varnish startup.

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
