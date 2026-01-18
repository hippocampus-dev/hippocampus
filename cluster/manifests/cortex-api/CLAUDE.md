# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repository contains Kubernetes manifests for the `cortex-api` service, an AI-driven API that provides OpenAI-compatible chat completion endpoints. The manifests use Kustomize for environment-specific configuration management.

## Common Development Commands

### Working with Manifests
- `kubectl apply -k overlays/dev/` - Apply development environment manifests
- `kubectl kustomize overlays/dev/` - Preview generated manifests
- `kubectl diff -k overlays/dev/` - Show differences before applying

### Related Application Commands
The actual application code is located at `/opt/hippocampus/cluster/applications/api/`:
- `make dev` - Start development server with auto-reload
- `make install` - Install dependencies via UV

## High-Level Architecture

### Kustomize Structure
- **base/** - Core resource definitions shared across environments
  - `deployment.yaml` - Pod template with 4 containers
  - `service.yaml` - Service exposure configuration
  - `horizontal_pod_autoscaler.yaml` - Auto-scaling rules
  - `pod_disruption_budget.yaml` - Availability constraints
  
- **overlays/dev/** - Development environment specific configurations
  - Istio integration (Gateway, VirtualService, DestinationRule)
  - Security policies (NetworkPolicy, PeerAuthentication)
  - Secret management via Vault
  - Environment-specific patches

### Key Design Patterns

1. **Multi-Container Pod Architecture**
   - `cortex-api` - Main application container
   - `redis-proxy` - Redis connection proxy
   - `exporter-merger` - Metrics aggregation
   - `chrome-devtools-protocol-server` - Browser automation support

2. **Security-First Configuration**
   - All containers run with `readOnlyRootFilesystem: true`
   - Non-root user enforcement
   - Minimal capabilities (drop ALL)
   - Network policies restricting traffic

3. **Istio Service Mesh Integration**
   - mTLS enforcement via PeerAuthentication
   - JWT validation with RequestAuthentication
   - Traffic management with VirtualService
   - Telemetry and distributed tracing

4. **High Availability Setup**
   - HorizontalPodAutoscaler for dynamic scaling
   - PodDisruptionBudget ensuring minimum replicas
   - TopologySpreadConstraints for pod distribution
   - Liveness and readiness probes

### Secret Management
Secrets are managed through Vault integration:
- GitHub tokens for API access
- OpenAI API keys
- Slack credentials
- Google service account keys

### Important Conventions
- Always maintain the existing YAML structure and key ordering
- Use Kustomize patches for environment-specific changes
- Security contexts are mandatory - never disable them
- All external traffic must go through Istio Gateway