# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying node-local-dns, a DNS caching solution that runs on each node in a Kubernetes cluster. It uses Kustomize for configuration management with a base configuration and environment-specific overlays.

## Common Development Commands

### Kustomize Commands
- `kustomize build base/` - Build the base manifests
- `kustomize build overlays/dev/` - Build the dev environment manifests with patches applied
- `kubectl apply -k overlays/dev/` - Deploy to dev environment using kustomize
- `kubectl diff -k overlays/dev/` - Preview changes before applying

### Validation and Testing
- `kubectl --dry-run=client -f <manifest.yaml>` - Validate individual manifest syntax
- `kustomize build overlays/dev/ | kubectl apply --dry-run=server -f -` - Server-side validation

## High-Level Architecture

### Structure
- **`base/`** - Core manifests and configurations:
  - `daemon_set.yaml` - DaemonSet running node-local-dns on every node
  - `service.yaml` - Service for upstream DNS (kube-dns)
  - `pod_disruption_budget.yaml` - PDB configuration for availability
  - `files/Corefile.base` - CoreDNS configuration template
  - `kustomization.yaml` - Base kustomization with ConfigMap generation

- **`overlays/dev/`** - Development environment customizations:
  - Prometheus monitoring annotations
  - Rolling update strategy configuration
  - Namespace assignment (kube-system)

### Key Components
1. **Node-local DNS Cache**: Runs on `169.254.20.10` and `10.96.0.10` (cluster DNS IP)
2. **CoreDNS Configuration**: Handles cluster.local, reverse DNS, and external domains
3. **Host Networking**: Runs with hostNetwork=true for node-level DNS caching
4. **Security Context**: Requires NET_ADMIN and NET_BIND_SERVICE capabilities for iptables management

### Important Configuration Details
- The DNS cache binds to special IPs: `169.254.20.10` (link-local) and `10.96.0.10` (cluster DNS)
- Uses placeholder values `__PILLAR__CLUSTER__DNS__` and `__PILLAR__UPSTREAM__SERVERS__` that are replaced at runtime
- Prometheus metrics exposed on port 9253
- Health checks on `http://169.254.20.10:8053/health`
- Requires privileged capabilities for network interface and iptables management