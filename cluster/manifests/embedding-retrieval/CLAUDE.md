# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The embedding-retrieval service is a Kubernetes manifest configuration for deploying the embedding-retrieval microservice to the Hippocampus platform. This directory contains Kustomize-based Kubernetes manifests organized in a base/overlays structure.

## Common Development Commands

### Kubernetes Deployment
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests without applying
- `kustomize build overlays/dev/` - Build manifests using kustomize directly

### Image Management
- Images are managed through kustomization.yaml files with SHA digests
- Base image: `ghcr.io/kaidotio/hippocampus/embedding-retrieval`
- Supporting images: tcp-proxy and exporter-merger for sidecar containers

## High-Level Architecture

### Manifest Structure
- **base/** - Core Kubernetes resources:
  - `deployment.yaml` - Main application deployment
  - `service.yaml` - Service definition for internal cluster access
  - `horizontal_pod_autoscaler.yaml` - Auto-scaling configuration
  - `pod_disruption_budget.yaml` - High availability constraints
  
- **overlays/dev/** - Development environment customizations:
  - `namespace.yaml` - Dedicated namespace definition
  - `gateway.yaml` - Istio Gateway for external access
  - `virtual_service.yaml` - Istio routing rules
  - `network_policy.yaml` - Network access controls
  - `patches/` - Kustomize patches for base resources
  - `qdrant/` - Vector database StatefulSet configuration

### Key Design Patterns
1. **Kustomize-based Configuration**: Separation of base resources and environment-specific overlays
2. **Istio Service Mesh Integration**: mTLS, observability, and traffic management
3. **Sidecar Pattern**: TCP proxy for connection pooling, exporter-merger for metrics aggregation
4. **Security Hardening**:
   - Non-root user (65532)
   - Read-only root filesystem
   - Dropped capabilities
   - Seccomp profiles
   - PodSecurityPolicy compliance

### Service Dependencies
- **Qdrant Vector Database**: Deployed as StatefulSet in same namespace
- **Embedding Gateway**: For OpenAI API proxy (http://embedding-gateway.embedding-gateway.svc.cluster.local:8080)
- **OpenTelemetry Collector**: For trace/metrics export (http://otel-agent.otel.svc.cluster.local:4317)
- **NFS Cache**: For FastEmbed model caching (host.minikube.internal:/srv/nfs/.cache)

### Environment Configuration
Key settings configured via patches:
- `DATASTORE=qdrant` - Vector store backend
- `QDRANT_HOST=embedding-retrieval-qdrant` - Local TCP proxy endpoint
- `QDRANT_COLLECTION=embedding-retrieval` - Collection name
- `QDRANT_REPLICATION_FACTOR=3` - High availability
- Secrets mounted from `embedding-retrieval` secret (managed by Vault)

### Deployment Notes
- Uses RollingUpdate strategy with surge/unavailable limits
- TopologySpreadConstraints for zone/node distribution
- Prometheus metrics exposed on port 8082 (merged from app + tcp-proxy)
- Resource requests set for cluster autoscaling
- Istio sidecar injection enabled with resource limits