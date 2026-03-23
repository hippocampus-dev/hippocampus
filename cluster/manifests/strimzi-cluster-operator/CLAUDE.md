# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kustomize-based deployment configuration for the Strimzi Kafka Operator in the Hippocampus Kubernetes platform. Strimzi provides a way to run Apache Kafka on Kubernetes using Kubernetes-native CRDs and operators.

## Common Development Commands

### Deploy to Kubernetes
```bash
# Direct deployment to dev environment
kubectl apply -k overlays/dev

# View the generated manifests without applying
kubectl kustomize overlays/dev

# Dry-run to see what would be applied
kubectl apply -k overlays/dev --dry-run=client -o yaml
```

### GitOps Deployment (Recommended)
This manifest is managed by ArgoCD in the Hippocampus platform. Changes pushed to Git will be automatically synchronized to the cluster.

## High-Level Architecture

### Directory Structure
- **`base/`** - Base Kustomization that references the upstream Strimzi operator manifest
  - Downloads from: https://github.com/strimzi/strimzi-kafka-operator/releases/download/0.45.0/strimzi-cluster-operator-0.45.0.yaml

- **`overlays/dev/`** - Development environment customizations
  - `namespace.yaml` - Creates the strimzi-cluster-operator namespace
  - `network_policy.yaml` - Network policies for security
  - `peer_authentication.yaml` - Istio mTLS configuration
  - `pod_disruption_budget.yaml` - PDB for high availability
  - `sidecar.yaml` - Istio sidecar configuration
  - `telemetry.yaml` - Telemetry and observability settings
  - `patches/` - Kustomize patches to modify the base deployment
    - `deployment.yaml` - Adds replicas, security context, topology spread constraints
    - `cluster_role_binding.yaml` - Adjusts RBAC bindings
    - `role_binding.yaml` - Additional role bindings

### Key Customizations
1. **Multi-replica deployment**: Configured for 2 replicas with proper topology spread
2. **Istio integration**: Sidecar injection enabled with proper annotations
3. **Security hardening**: SecurityContext, read-only root filesystem, non-root user
4. **Resource management**: Memory/CPU limits removed via inline patch
5. **Namespace watching**: Configured to watch all namespaces (STRIMZI_NAMESPACE="*")

### Integration Points
- **Istio**: Service mesh integration with mTLS and telemetry
- **Prometheus**: Metrics exposed on port 8080
- **Network Policies**: Configured for secure inter-service communication
- **Pod Disruption Budget**: Ensures availability during updates

## Important Notes
- The operator watches all namespaces by default in the dev overlay
- Istio sidecar injection excludes Zookeeper (2181) and Kafka (9091) ports to avoid conflicts
- Uses Kubernetes 1.25+ topology spread constraints for better pod distribution
- Follows the Hippocampus project's Kubernetes manifest standards