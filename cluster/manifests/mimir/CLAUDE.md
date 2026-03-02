# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying Grafana Mimir, a horizontally scalable, highly available, multi-tenant, long-term storage for Prometheus metrics. The deployment uses Kustomize for configuration management and is designed to be deployed via ArgoCD in a GitOps workflow.

## Common Development Commands

### Manifest Generation and Validation
- `kustomize build overlays/dev` - Generate complete Kubernetes manifests for dev environment
- `kustomize build overlays/dev | kubectl apply --dry-run=client -f -` - Validate generated manifests
- `kustomize build overlays/dev | kubectl diff -f -` - Show differences between current and desired state

### Deployment Commands
- **ArgoCD (Recommended)**: Push changes to Git and ArgoCD will automatically sync
- **Manual Apply**: `kustomize build overlays/dev | kubectl apply -f -`
- **Manual Delete**: `kustomize build overlays/dev | kubectl delete -f -`

### Development Workflow
1. Modify manifests in `base/` for core changes or `overlays/dev/` for environment-specific changes
2. Test manifest generation: `kustomize build overlays/dev`
3. Commit and push changes - ArgoCD will automatically deploy

## High-Level Architecture

### Directory Structure
- **`base/`** - Core Kubernetes resources shared across environments
  - Contains Deployments, StatefulSets, Services, HPA, and PDB definitions
  - Configuration files for NGINX proxy, alert rules, and recording rules
- **`overlays/dev/`** - Development environment customizations
  - Additional resources: etcd, Memcached, MinIO
  - Patches for resource limits and Istio integration
  - Mimir configuration file with environment-specific settings

### Component Architecture
1. **Core Mimir Components**:
   - **Distributor**: Receives incoming metrics from Prometheus
   - **Ingester**: Stores recent metrics in memory/disk
   - **Querier**: Executes PromQL queries
   - **Query Frontend**: Caches and accelerates queries
   - **Query Scheduler**: Distributes queries to queriers
   - **Compactor**: Compacts TSDB blocks in object storage
   - **Store Gateway**: Queries historical data from object storage
   - **Ruler**: Evaluates recording and alerting rules
   - **Alertmanager**: Manages alerts (multi-tenant)

2. **Supporting Services**:
   - **MinIO**: S3-compatible object storage for TSDB blocks
   - **Memcached**: Caching layer for query results
   - **etcd**: High-availability tracker for distributor
   - **NGINX Proxy**: Reverse proxy for external access

### Key Design Patterns
1. **Kustomize-based Configuration**:
   - Base resources define core functionality
   - Overlays apply environment-specific modifications via patches
   - ConfigMaps generated from files with immutability

2. **High Availability**:
   - StatefulSets for components requiring stable identity
   - Memberlist for distributed coordination (gossip protocol)
   - Replication factors configured via Kustomize replacements

3. **Security Hardening**:
   - Non-root containers (UID 65532)
   - Read-only root filesystem
   - Dropped capabilities
   - Seccomp profiles enabled
   - Network policies for traffic isolation

4. **Istio Service Mesh Integration**:
   - Automatic sidecar injection
   - mTLS via PeerAuthentication
   - Telemetry configuration for metrics

5. **GitOps Deployment**:
   - ArgoCD Application manages the deployment
   - Auto-sync enabled with pruning
   - Sync waves for ordered initialization (Jobs run first)

### Configuration Management
- Main Mimir configuration in `overlays/dev/files/mimir.yaml`
- Environment variable expansion supported (e.g., `${HOSTNAME}`)
- Alert and recording rules in `base/files/`
- NGINX proxy configuration for unified access point

### Storage Architecture
- **Object Storage**: MinIO for TSDB blocks
- **Local Storage**: Persistent volumes for ingester WAL and alertmanager configuration
- **Cache**: Memcached for query result caching
- **Coordination**: etcd for HA tracker, memberlist for rings
- **Temporary Storage**: Alertmanager requires `/tmp` emptyDir (Memory medium) for API configuration validation operations when running with read-only root filesystem

### Alertmanager Configuration
- Uses `filesystem` backend for alertmanager_storage (writable, supports silences)
- Fallback configuration via ConfigMap (`alertmanager.yaml`) mounted to `/etc/mimir/alertmanager/anonymous/`
- Alertmanager state (silences, notifications) persisted to PVC at `/var/alertmanager`
- Notifications sent to alerthandler service via webhook
