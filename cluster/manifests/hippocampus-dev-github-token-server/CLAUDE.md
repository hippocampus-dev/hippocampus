# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains the Kubernetes manifests for deploying the github-token-server in the hippocampus-dev environment. The github-token-server is a microservice that generates GitHub access tokens for GitHub Actions workflows by authenticating as a GitHub App. This overlay applies environment-specific configurations to the base github-token-server application.

## Common Development Commands

### Kustomize Operations
- `kubectl kustomize overlays/dev/github-token-server/` - Build and preview the final manifests
- `kubectl apply -k overlays/dev/github-token-server/` - Deploy to Kubernetes cluster
- `kubectl diff -k overlays/dev/github-token-server/` - Show differences between current deployment and new manifests

### Validation
- `kubectl apply -k overlays/dev/github-token-server/ --dry-run=client` - Validate manifests without applying
- `kubectl kustomize overlays/dev/github-token-server/ | kubectl apply --dry-run=server -f -` - Server-side validation

## High-Level Architecture

### Directory Structure
```
overlays/dev/github-token-server/
├── kustomization.yaml          # Main kustomization configuration
├── kustomizeconfig.yaml        # Custom kustomize configurations
├── patches/                    # Deployment patches for dev environment
│   ├── deployment.yaml         # Dev-specific deployment configurations
│   ├── horizontal_pod_autoscaler.yaml
│   ├── pod_disruption_budget.yaml
│   └── service.yaml
├── authorization_policy.yaml   # Istio authorization rules
├── envoy_filter.yaml          # Envoy proxy configurations
├── peer_authentication.yaml    # mTLS configuration
├── request_authentication.yaml # JWT authentication
├── secrets_from_vault.yaml    # Vault secret generator
├── sidecar.yaml              # Istio sidecar configuration
└── telemetry.yaml            # Observability configuration
```

### Key Configurations

1. **Base Resources**: Inherits from `/utilities/github-token-server` with standard Kubernetes resources (Deployment, Service, HPA, PDB)

2. **Istio Integration**:
   - mTLS enforcement via PeerAuthentication
   - JWT validation via RequestAuthentication
   - Authorization policies for access control
   - Custom Envoy filters for advanced routing
   - Sidecar configuration for traffic management

3. **Observability**:
   - OpenTelemetry tracing with OTLP endpoint configuration
   - Pyroscope continuous profiling integration
   - Custom telemetry configuration for metrics collection

4. **Security**:
   - Secrets loaded from Vault via generator
   - GitHub App private key injected as environment variable
   - Service runs as non-root user (65532) with read-only filesystem

5. **Deployment Strategy**:
   - Rolling update with maxSurge: 25%, maxUnavailable: 1
   - Topology spread constraints for HA across nodes and zones
   - Resource requests: 5m CPU

### Environment-Specific Values
- **Namespace**: hippocampus-dev (applied via parent kustomization)
- **Name Prefix**: hippocampus-dev-
- **GitHub App Client ID**: Iv23li6gN5ht5DX51aVc
- **OpenTelemetry Endpoint**: http://otel-agent.otel.svc.cluster.local:4317
- **Pyroscope Endpoint**: http://pyroscope-distributor.pyroscope.svc.cluster.local:4040