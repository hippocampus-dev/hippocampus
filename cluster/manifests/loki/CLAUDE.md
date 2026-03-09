# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes manifests for deploying Loki, a horizontally-scalable, multi-tenant log aggregation system inspired by Prometheus. The deployment uses Kustomize for configuration management with a base/overlay structure.

## Common Development Commands

### Building and Applying Manifests
```bash
# Build the manifests for dev environment
kubectl kustomize overlays/dev

# Apply to cluster
kubectl apply -k overlays/dev

# Delete from cluster
kubectl delete -k overlays/dev
```

### Validating Changes
```bash
# Validate YAML syntax
kubectl kustomize overlays/dev | kubectl apply --dry-run=client -f -

# Check generated manifests
kubectl kustomize overlays/dev > generated.yaml
```

## Architecture

### Directory Structure
- **`base/`** - Core Loki components and configuration
  - Contains deployments for: distributor, query-frontend, query-scheduler, querier, proxy
  - Contains statefulsets for: compactor, ingester, index-gateway, ruler
  - Includes HPA, PDB, and service definitions
  
- **`overlays/dev/`** - Development environment overlay
  - Adds memcached and minio as dependencies
  - Configures Loki with specific retention, limits, and storage settings
  - Includes Istio integration (sidecars, telemetry, peer authentication)

### Component Architecture

Loki is deployed in microservices mode with the following components:

1. **Write Path**
   - `distributor` - Receives logs and distributes to ingesters
   - `ingester` - Stores logs in memory/disk before flushing to object storage

2. **Read Path**
   - `query-frontend` - Handles queries, splits them, and caches results
   - `query-scheduler` - Manages query scheduling and coordination
   - `querier` - Executes queries against ingesters and storage

3. **Storage Components**
   - `compactor` - Compacts and manages retention of stored logs
   - `index-gateway` - Provides index queries for storage backends
   - `ruler` - Evaluates alerting and recording rules

4. **Supporting Services**
   - `proxy` (nginx) - Routes requests to appropriate components
   - `memcached` - Caches query results and chunks
   - `minio` - S3-compatible object storage backend

### Key Configuration Patterns

1. **Memberlist** - Used for component discovery and coordination
2. **Resource Management** - All components use GOMAXPROCS and GOMEMLIMIT from resource limits
3. **Security** - Non-root containers, read-only filesystems, dropped capabilities
4. **Observability** - Jaeger tracing integration, Istio telemetry
5. **High Availability** - PodDisruptionBudgets, HorizontalPodAutoscalers

### Storage Configuration
- Uses S3 (MinIO) for object storage
- Schema versions: v11 (legacy), v13 (current)
- Index storage: TSDB format
- Retention: 24h default with stream-specific overrides

## Important Notes
- All manifests follow strict Kubernetes security practices
- Components are designed to be stateless except for ingesters and storage
- The proxy component uses nginx to route traffic based on URL paths
- Memcached is prefixed with `loki-` to avoid conflicts
- Istio sidecars are injected for all components in dev overlay