# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the OpenTelemetry Collector deployment in the Hippocampus platform. The OpenTelemetry Collector is responsible for receiving, processing, and exporting telemetry data (traces, metrics, logs) across the platform's microservices.

## Common Development Commands

### Kustomize Commands
- `kustomize build base/` - Build base manifests
- `kustomize build overlays/dev/` - Build development environment manifests
- `kubectl apply -k overlays/dev/` - Apply development manifests to cluster
- `kubectl diff -k overlays/dev/` - Preview changes before applying

### Validation Commands
- `kubectl --dry-run=client -f base/deployment.yaml` - Validate deployment manifest
- `kustomize build overlays/dev/ | kubectl apply --dry-run=server -f -` - Server-side validation

## High-Level Architecture

### Directory Structure
- **`base/`** - Core Kubernetes resources shared across all environments:
  - `deployment.yaml` - OTel Collector container configuration
  - `service.yaml` - Service exposing collector endpoints
  - `horizontal_pod_autoscaler.yaml` - Auto-scaling configuration
  - `pod_disruption_budget.yaml` - High availability settings

- **`overlays/dev/`** - Development environment customizations:
  - `files/config.yaml` - OTel Collector pipeline configuration
  - `patches/` - Resource modifications for dev environment
  - Istio integration manifests (sidecar, telemetry, peer authentication)
  - Network policies for security

### Key Design Patterns

1. **Kustomize-based Configuration**: Uses base+overlay pattern for environment-specific configurations
2. **Security-First Design**: 
   - Non-root container execution
   - Read-only root filesystem
   - All capabilities dropped
   - Strict network policies
3. **Istio Service Mesh Integration**: mTLS communication between services
4. **High Availability**: Pod disruption budgets and horizontal autoscaling

### OpenTelemetry Pipeline Configuration

The collector pipeline (in `overlays/dev/files/config.yaml`) follows this flow:
1. **Receivers**: OTLP gRPC endpoint on port 4317
2. **Processors**: 
   - Sensitive data hashing (db.statement attributes)
   - Batch processing (8192 batch size, 200ms timeout)
3. **Exporters**: Tempo backend for distributed tracing

### Integration Points

- **Tempo**: Trace data export endpoint at `http://tempo:4317`
- **Service Mesh**: Istio sidecar injection for secure communication
- **Monitoring**: Exposes metrics on port 8888 for Prometheus scraping