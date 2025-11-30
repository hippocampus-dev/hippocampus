# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying Pyroscope (continuous profiling tool) using Kustomize. It's part of the Hippocampus platform and follows a base/overlays structure for managing different environments.

## Common Development Commands

### Kustomize Operations
- `kustomize build overlays/dev` - Build manifests for development environment
- `kustomize build overlays/dev | kubectl apply -f -` - Deploy to Kubernetes cluster
- `kustomize build overlays/dev | kubectl diff -f -` - Preview changes before applying
- `kustomize build overlays/dev | kubectl delete -f -` - Remove all resources

### Validation and Testing
- `kustomize build overlays/dev | kubectl --dry-run=client apply -f -` - Validate manifests without applying
- `kubectl get pods -n pyroscope` - Check pod status
- `kubectl logs -n pyroscope -l app.kubernetes.io/name=pyroscope` - View logs across all Pyroscope components

### Development with Skaffold (from project root)
- `skaffold dev --port-forward` - Deploy with hot reload and port forwarding

## High-Level Architecture

### Directory Structure
- **`base/`** - Core Kubernetes resources shared across environments:
  - Deployments: distributor, query-frontend, query-scheduler, querier
  - StatefulSets: compactor, ingester, store-gateway
  - Services, HPA, and PodDisruptionBudget configurations
  
- **`overlays/dev/`** - Development environment specific configurations:
  - `files/pyroscope.yaml` - Main Pyroscope configuration
  - `minio/` - S3-compatible object storage for data persistence
  - Istio integration (PeerAuthentication, Sidecar, Telemetry)
  - Resource patches and network policies

### Key Components
1. **Distributor** - Receives profiling data from clients
2. **Ingester** - Processes and stores recent profiling data
3. **Querier** - Handles read queries across all data
4. **Query Frontend** - Caches and accelerates queries
5. **Query Scheduler** - Distributes queries to queriers
6. **Compactor** - Compacts blocks and downsamples old data
7. **Store Gateway** - Queries historical data from object storage

### Configuration Details
- Uses memberlist for cluster coordination (no external dependencies)
- MinIO as S3-compatible backend storage
- 7-day data retention with 10-minute block rotation
- Istio service mesh integration for security and observability
- Multi-tenancy disabled for simplicity