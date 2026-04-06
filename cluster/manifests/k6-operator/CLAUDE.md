# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying the k6-operator, which enables running Grafana k6 load tests on Kubernetes using Custom Resource Definitions (CRDs). The deployment uses Kustomize for configuration management with a base/overlay structure.

## Common Development Commands

### Deployment
```bash
# Deploy to development environment
kubectl apply -k overlays/dev

# Deploy base resources only (requires namespace specification)
kubectl apply -k base -n <namespace>

# Verify deployment
kubectl get deployment -n k6-operator k6-operator
kubectl get crd testruns.k6.io
```

### Testing and Validation
```bash
# Validate kustomization
kubectl kustomize overlays/dev --enable-helm | kubectl apply --dry-run=client -f -

# Check operator logs
kubectl logs -n k6-operator deployment/k6-operator -f

# List test runs
kubectl get testruns -A
```

### Maintenance
```bash
# Update operator image (edit base/kustomization.yaml digest)
# Then redeploy:
kubectl apply -k overlays/dev

# Check operator metrics
kubectl port-forward -n k6-operator deployment/k6-operator 8080:8080
# Visit http://localhost:8080/metrics
```

## High-Level Architecture

### Directory Structure
- **base/** - Core k6-operator resources
  - CRDs: K6 (deprecated), TestRun, PrivateLoadZone
  - RBAC: ClusterRole, ClusterRoleBinding, Role, RoleBinding
  - Deployment with security hardening (non-root, read-only FS)
  - ServiceAccount and PodDisruptionBudget
  
- **overlays/dev/** - Development environment configuration
  - Namespace: k6-operator
  - Istio integration (sidecar injection, mTLS, telemetry)
  - NetworkPolicy for secure communication
  - Patches for 2-replica HA deployment

### Key Design Patterns
1. **Kustomize-based deployment** - Base resources with environment-specific overlays
2. **Image mirroring** - Uses internal registry mirror (ghcr.io/hippocampus-dev/hippocampus/mirror/)
3. **Security hardening** - Non-root user (65532), dropped capabilities, read-only root filesystem
4. **High availability** - 2 replicas with PodDisruptionBudget in dev environment
5. **Istio service mesh integration** - Automatic sidecar injection, metrics, and tracing

### TestRun Resource Usage
k6 test scripts are typically stored in `/opt/hippocampus/cluster/applications/*/k6/` directories. Example TestRun:
```yaml
apiVersion: k6.io/v1alpha1
kind: TestRun
metadata:
  name: load-test
spec:
  script:
    configMap:
      name: k6-test-script
      file: script.js
  parallelism: 4
  arguments: --vus 10 --duration 30s
```

### Integration Points
- **Prometheus metrics** - Exposed on port 8080 at /metrics
- **Istio telemetry** - OpenTelemetry traces and Prometheus metrics via Istio
- **RBAC permissions** - Manages Jobs, Deployments, Services, Pods, Secrets across namespaces
- **Leader election** - Uses Lease resources for operator HA