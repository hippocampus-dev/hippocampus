# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying fluentd-delayed-unlink, an eBPF-based DaemonSet that prevents race conditions between Kubernetes log rotation and Fluentd log processing. The manifests follow a Kustomize-based structure with base resources and environment-specific overlays.

## Common Development Commands

### Building and Applying Manifests
- `kustomize build base/` - Build base manifests
- `kustomize build overlays/dev/` - Build dev environment manifests with all patches and overlays
- `kubectl apply -k overlays/dev/` - Apply dev manifests to cluster
- `kubectl diff -k overlays/dev/` - Preview changes before applying

### Validation
- `kubectl --dry-run=client -f <(kustomize build overlays/dev/) apply` - Validate generated manifests

## High-Level Architecture

### Manifest Structure

1. **Base Layer** (`base/`)
   - `daemon_set.yaml` - Core DaemonSet definition running on all nodes
   - `pod_disruption_budget.yaml` - PDB configuration for availability
   - `kustomization.yaml` - Base Kustomize configuration with image digests

2. **Dev Overlay** (`overlays/dev/`)
   - `namespace.yaml` - Dedicated namespace definition
   - `network_policy.yaml` - Network policies for pod communication
   - `peer_authentication.yaml` - Istio mTLS configuration
   - `sidecar.yaml` - Istio sidecar configuration limiting egress to istiod and otel-agent
   - `telemetry.yaml` - Istio telemetry configuration
   - `patches/` - Environment-specific patches for base resources

### Key Configuration Details

- **DaemonSet**: Runs privileged with host path mounts to `/sys/kernel/debug` and `/var/log`
- **hostPID**: Required (`hostPID: true`) for eBPF to see host PIDs - the application reads `/proc/self/status` NSpid to translate container PID namespace to host PID namespace
- **Security**: Uses RuntimeDefault seccomp profile, drops all capabilities except required for eBPF
- **Observability**: Exposes metrics on port 8080, integrated with Prometheus scraping
- **Istio Integration**: Sidecar injection enabled with resource limits and mTLS
- **Priority**: Uses `system-node-critical` priority class
- **Tolerations**: Tolerates all node taints to ensure deployment on all nodes

### Image Management

Images are managed through Kustomize's image transformation:
- Base image: `ghcr.io/hippocampus-dev/hippocampus/fluentd-delayed-unlink`
- Pinned to specific SHA256 digests in base kustomization.yaml
