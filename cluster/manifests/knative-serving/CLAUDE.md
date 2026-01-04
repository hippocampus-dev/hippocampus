# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kustomize-based Kubernetes manifests for deploying Knative Serving with Istio integration. It follows a layered configuration approach with base manifests from upstream Knative and environment-specific overlays.

## Common Development Commands

### Deploy and manage Knative Serving
```bash
# Build manifests (dry-run to see what will be deployed)
kubectl kustomize overlays/dev

# Deploy to cluster
kubectl apply -k overlays/dev

# Delete from cluster
kubectl delete -k overlays/dev

# Verify deployment
kubectl get pods -n knative-serving
kubectl get deployments -n knative-serving
```

### Test Knative Service deployment
```bash
# Deploy a test service
kubectl apply -f - <<EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: hello
  namespace: default
spec:
  template:
    spec:
      containers:
      - image: gcr.io/knative-samples/helloworld-go
        env:
        - name: TARGET
          value: "World"
EOF

# Check service status
kubectl get ksvc hello
```

## High-Level Architecture

### Directory Structure
- **`base/`** - References upstream Knative Serving v1.20.1 manifests
- **`overlays/dev/`** - Development environment customizations:
  - Istio integration (net-istio v1.20.1)
  - Security policies (NetworkPolicy, PeerAuthentication)
  - Resource patches for all core components
  - Telemetry and observability configurations

### Key Customizations
1. **Istio Integration** - All workloads have sidecar injection enabled with resource limits
2. **Security** - Default-deny NetworkPolicy with explicit allow rules for required communication
3. **Observability** - Prometheus metrics and Istio telemetry configurations
4. **Auto-scaling** - HPA configurations for webhook and activator components
5. **Development Domain** - Configured for `*.minikube.127.0.0.1.nip.io` domains

### Configuration Patches
- **ConfigMaps**: Domain settings, feature flags, GC policies, Istio gateway configurations
- **Deployments**: Sidecar injection, resource limits, topology spread constraints
- **HPA**: CPU-based autoscaling (1-5 replicas, 80% CPU target)
- **Namespace**: Labels for Istio injection and pod security standards

### Prerequisites
- Kubernetes cluster with Istio installed
- kubectl with Kustomize support (built-in for kubectl v1.14+)
- For development: Minikube or similar local Kubernetes environment
