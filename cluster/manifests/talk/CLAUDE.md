# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the Talk application - a real-time chat interface that connects to OpenAI's GPT-4 Realtime API via WebSocket. The manifests use Kustomize for environment-specific configuration and Istio for service mesh capabilities.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl delete -k overlays/dev/` - Remove from development environment
- `kustomize build overlays/dev/` - Generate final manifests without applying
- `kubectl rollout restart deployment/talk -n talk` - Restart the deployment
- `kubectl logs -f deployment/talk -n talk` - View application logs

### Validation and Testing
- `kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -` - Validate manifests
- `istioctl analyze -n talk` - Analyze Istio configuration
- `kubectl describe hpa talk -n talk` - Check autoscaling status

## High-Level Architecture

### Directory Structure
```
talk/
├── base/                           # Base Kustomize configuration
│   ├── deployment.yaml            # Core deployment specification
│   ├── horizontal_pod_autoscaler.yaml  # HPA configuration
│   ├── pod_disruption_budget.yaml      # PDB for high availability
│   ├── service.yaml               # ClusterIP service
│   └── kustomization.yaml         # Base kustomization
└── overlays/
    └── dev/                       # Development environment overlay
        ├── files/host.js          # Environment-specific host configuration
        ├── gateway.yaml           # Istio Gateway for ingress
        ├── virtual_service.yaml   # Istio routing rules
        ├── namespace.yaml         # Namespace with Istio injection
        ├── patches/               # Strategic merge patches
        └── *.yaml                 # Other Istio and security configs
```

### Key Components

1. **Base Configuration**:
   - Deployment with nginx serving static files
   - Horizontal Pod Autoscaler (2-10 replicas based on CPU/memory)
   - Pod Disruption Budget (minimum 1 available pod)
   - ClusterIP service on port 80

2. **Development Overlay**:
   - Namespace with Istio sidecar injection enabled
   - Gateway exposing the app at `talk.minikube.127.0.0.1.nip.io`
   - VirtualService for HTTP routing
   - Network policies for security
   - PeerAuthentication for mTLS
   - Telemetry configuration for observability
   - ConfigMap replacement for environment-specific host.js

3. **Istio Integration**:
   - Automatic sidecar injection via namespace label
   - mTLS enabled for service-to-service communication
   - Gateway for external access
   - Telemetry for distributed tracing

### Deployment Workflow

1. **Local Development**:
   - Application code is in `/opt/hippocampus/cluster/applications/talk/`
   - Build Docker image: `make docker-build` (in application directory)
   - Deploy to Kubernetes: `kubectl apply -k overlays/dev/`

2. **Configuration Updates**:
   - Base changes affect all environments
   - Overlay changes are environment-specific
   - Use strategic merge patches for fine-grained modifications

3. **Environment-Specific Files**:
   - `overlays/dev/files/host.js` replaces the default host configuration
   - ConfigMap generation automatically handles file updates

## Important Notes

- The application requires cortex-api to be running for WebSocket proxy to OpenAI
- Istio must be installed in the cluster for proper functionality
- The dev overlay uses nip.io for DNS resolution without configuration
- HPA scales based on 70% CPU or 80% memory utilization
- Network policies restrict ingress to Istio gateways only