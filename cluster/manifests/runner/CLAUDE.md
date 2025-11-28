# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying GitHub Actions self-hosted runners managed by the github-actions-runner-controller. The manifests use Kustomize for templating and support multiple environments through overlays.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests without applying
- `kubectl delete -k overlays/dev/` - Remove all resources from development environment

### Controller Development (in github-actions-runner-controller directory)
- `make all` - Generate CRDs from Go types using controller-gen
- `make dev` - Create Kind cluster and run Skaffold for hot-reload development
- `skaffold dev --port-forward` - Run controller with hot-reload in existing cluster

## High-Level Architecture

### Directory Structure
- **base/** - Base Runner CRD configuration
  - `runner.yaml` - Runner custom resource definition
  - `kustomization.yaml` - Base Kustomize configuration with image digest
  - `kustomizeconfig.yaml` - Kustomize configuration for CRD transformations

- **overlays/dev/** - Development environment specific configurations
  - `namespace.yaml` - Namespace definition
  - `network_policy.yaml` - Restrictive network policies
  - `cluster_role_binding.yaml` - ClusterRoleBinding granting ClusterRole `view` to runner ServiceAccount
  - `service_account.yaml` - ServiceAccount for runner pods
  - `service_entry.yaml` - Istio ServiceEntry for external access
  - `secrets_from_vault.yaml` - Vault integration for Docker credentials
  - `patches/runner.yaml` - Environment-specific Runner patches

### Key Design Decisions

1. **Istio Ambient Mesh**
   - Uses `istio.io/dataplane-mode: ambient` instead of sidecar proxy
   - Avoids conflicts with Kaniko (cannot run with UID 1337)
   - Provides service mesh features without init container issues

2. **Vault Integration**
   - Uses SecretsFromVault CRD to fetch Docker registry credentials
   - Retrieves `DOCKER_CONFIG_JSON` from `/kv/data/runner` path
   - Avoids storing sensitive data in Kubernetes Secrets

3. **Network Security**
   - Default deny all traffic with NetworkPolicy
   - Only allows Prometheus metrics collection (port 15020)
   - ServiceEntry permits access to:
     - GitHub API (api.github.com, github.com)
     - Docker registries (registry-1.docker.io, ghcr.io)
     - Ubuntu archives for package installation

4. **Resource Configuration**
   - Runner spec defines:
     - Image: `ghcr.io/kaidotio/hippocampus/runner`
     - Owner: `kaidotio`
     - Repo: `hippocampus`
   - Development overlay adds labels and annotations for Istio integration

5. **RBAC Configuration**
   - ServiceAccount `runner` is assigned to runner pods via `template.spec.serviceAccountName`
   - ClusterRoleBinding grants ClusterRole `view` to the ServiceAccount
   - Allows runners to read Kubernetes resources cluster-wide for CI/CD tasks

### Dependencies

- **github-actions-runner-controller** - Manages Runner lifecycle
- **Istio** - Service mesh (ambient mode)
- **Vault** - Secret management
- **Kustomize** - Manifest templating
- **Kaniko** - Container image building (used by controller)

### Workflow

1. Apply manifests using Kustomize
2. Controller watches for Runner resources
3. Controller creates:
   - ConfigMap with Dockerfile
   - Deployment with Kaniko init container and runner container
   - Secret with GitHub token (if using GitHub App auth)
4. Kaniko builds custom runner image and pushes to registry
5. Runner container starts and registers with GitHub Actions
6. Vault provides Docker credentials through SecretsFromVault operator
7. Network policies and ServiceEntries control traffic flow