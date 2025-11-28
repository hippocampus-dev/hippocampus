# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

litestream-hook is a Kubernetes admission webhook that automatically injects Litestream (SQLite streaming replication) into Pods. It intercepts Pod creation, checks for specific annotations, and modifies Pod specs to add init containers for database restoration and a sidecar container for continuous replication to S3-compatible storage.

## Common Development Commands

### Primary Development
- `make dev` - Runs skaffold dev with hot-reload and port-forwarding

### Testing Workflow
```bash
# Deploy test environment (MinIO + test pod)
kubectl apply -f examples/minio.yaml
kubectl apply -f examples/mc.yaml
kubectl apply -f examples/pod.yaml
```

### Build Commands
- Docker build uses multi-stage Dockerfile with distroless final image
- CGO is disabled for static binary compilation
- Supports LD_FLAGS customization at build time

## High-Level Architecture

### Core Components

1. **Webhook Handler** (`main.go`):
   - Uses controller-runtime framework
   - Registers mutating webhook at `/mutate`
   - Processes admission requests to inject Litestream

2. **Injection Logic**:
   - **Init Containers**: 
     - `litestream-hook-init`: Generates Litestream config
     - `litestream-hook-restore-{n}`: Restores each database from S3
   - **Sidecar Container**: `litestream-hook-replicate` runs continuous replication

3. **Configuration via Pod Annotations**:
   - API group prefix customizable via VARIANT env var (default: `litestream-hook.kaidotio.github.io`)
   - Key annotations: inject, storage, image, bucket, secret, path, endpoint
   - Supports multiple databases via comma-separated paths

### Deployment Architecture

1. **Kubernetes Resources**:
   - Deployment with 2 replicas for HA
   - Service on port 9443 for webhook
   - MutatingWebhookConfiguration for API registration
   - Uses cert-manager for TLS certificates
   - PodDisruptionBudget for availability guarantees

2. **Security Design**:
   - Runs as non-root user (65532)
   - Read-only root filesystem
   - Distroless container image
   - All capabilities dropped
   - RuntimeDefault seccomp profile

3. **Storage Design**:
   - Memory-backed emptyDir for Litestream config
   - Disk-backed emptyDir for SQLite databases
   - Configurable storage size via annotation

### Development Workflow

1. Use `make dev` for local development with automatic rebuilds
2. Test with examples/ manifests against local MinIO
3. Skaffold handles build, push, and deployment
4. Development patches in skaffold/ directory override production settings