# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes ValidatingAdmissionPolicy implementation that provides declarative admission control for a Kubernetes cluster. Unlike traditional webhook-based admission controllers, this uses the native policy-based approach introduced in Kubernetes 1.26+.

The project defines validation policies using CEL (Common Expression Language) to enforce best practices and security requirements across the cluster without requiring any running webhook servers.

## Common Development Commands

### Validation and Testing
```bash
# Validate Kubernetes manifests
kubectl apply --dry-run=client -f base/validating_admission_policy.yaml
kubectl apply --dry-run=client -f base/validating_admission_policy_binding.yaml

# Build with Kustomize
kustomize build base/
kustomize build overlays/dev/

# Apply to cluster (requires appropriate permissions)
kubectl apply -k base/
kubectl apply -k overlays/dev/

# Check policy status
kubectl get validatingadmissionpolicies
kubectl get validatingadmissionpolicybindings
```

### Debugging Policies
```bash
# View policy evaluation results
kubectl describe validatingadmissionpolicy <policy-name>

# Check audit logs for policy violations
kubectl logs -n kube-system -l component=kube-apiserver | grep admission
```

## High-Level Architecture

### Policy Structure
Each ValidatingAdmissionPolicy consists of:
1. **Match Constraints**: Defines which resources the policy applies to
2. **Variables**: CEL expressions that compute intermediate values
3. **Validations**: CEL expressions that must evaluate to true for admission

### Implemented Policies

1. **PodDisruptionBudget Validation** (`poddisruptionbudgets.validating.kaidotio.github.io`)
   - Ensures `maxUnavailable` is set to a positive value
   - Applies to all PodDisruptionBudget CREATE/UPDATE operations

2. **Pod Volume Validation** (`pods.validating.kaidotio.github.io`)
   - Ensures all defined volumes are mounted in containers or initContainers
   - Excludes istio-system namespace and waypoint gateway pods
   - Uses complex CEL expressions with multiple variables

3. **Service Port Naming** (`services.validating.kaidotio.github.io`)
   - Enforces port names start with: grpc, http, tls, tcp, or udp
   - Excludes kube-system and argocd namespaces

4. **DaemonSet Priority Class** (`daemonsets.validating.kaidotio.github.io`)
   - Ensures DaemonSets in kube-system use 'system-node-critical' priority class
   - Critical for cluster stability

### Key Design Patterns

1. **Namespace Exclusions**: System namespaces are excluded via `namespaceSelector` in bindings
2. **Fail-Fast Policy**: All policies use `failurePolicy: Fail` for security
3. **Dual Actions**: All bindings use both `Deny` and `Audit` actions
4. **Variable-Based Validation**: Complex logic is broken down using intermediate variables

### CEL Expression Patterns
- List operations: `map()`, `filter()`, `all()`, `exists()`
- Conditional checks: `has()` for optional fields
- String matching: `find()` with regex patterns
- Set operations: `in` for membership checks

## Deployment Workflow

1. **Kustomize Structure**:
   - `base/`: Core policy definitions
   - `overlays/dev/`: Environment-specific configurations (currently inherits base)

2. **GitOps Integration**: 
   - Deployed via ArgoCD to kube-system namespace
   - Wave: -100 (early deployment)
   - Auto-sync and self-healing enabled

3. **Testing New Policies**:
   - Add policy definition to `base/validating_admission_policy.yaml`
   - Add binding to `base/validating_admission_policy_binding.yaml`
   - Test with dry-run before applying
   - Monitor audit logs for unexpected rejections