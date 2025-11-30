# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Grafana deployment manifest for Kubernetes using Kustomize and Jsonnet. It generates Grafana dashboards programmatically using Grafonnet library and deploys Grafana with pre-configured dashboards for monitoring Kubernetes clusters.

## Common Development Commands

### Dashboard Generation
- `make all` - Installs Jsonnet dependencies and generates all dashboard JSON files from Jsonnet sources
- `make install` - Installs Jsonnet dependencies using jsonnet-bundler in Docker
- Generated dashboards are prefixed with `zz_generated.` to indicate they are auto-generated

### Build Process
The Makefile uses Docker to run jsonnet-builder, which:
1. Installs dependencies from `jsonnetfile.json` (Grafonnet and XTD libraries)
2. Compiles `.jsonnet` files to `.json` dashboard files
3. Stores generated files in `base/dashboards/` directories

## Architecture

### Directory Structure
- `jsonnet/` - Source Jsonnet files for dashboards
  - `common.libsonnet` - Shared functions for dashboard generation
  - `base/dashboards/kubernetes/` - Kubernetes monitoring dashboards
  - `base/dashboards/other/` - Additional dashboards (overview, lighthouse)
- `base/` - Base Kustomize resources
  - `dashboards/` - Generated JSON dashboard files (DO NOT EDIT - generated from Jsonnet)
  - Core Kubernetes resources (deployment, service, HPA, PDB)
- `overlays/dev/` - Development environment overlay with Istio integration

### Key Components
1. **Grafana Container** - Main Grafana instance with anonymous access enabled
2. **MCP Container** - MCP (Model Context Protocol) sidecar for Grafana integration
3. **Dashboard Provisioning** - ConfigMaps containing dashboard JSON files mounted into Grafana

### Dashboard Categories
- **Kubernetes Dashboards**: cluster, node, namespace, workload, pod, volume, cronjob
- **Other Dashboards**: overview (default home), lighthouse

### Development Workflow
1. Edit Jsonnet files in `jsonnet/` directory
2. Run `make all` to regenerate dashboard JSON files
3. Deploy using Kustomize: `kubectl apply -k overlays/dev/`

### Important Patterns
- All generated files use `zz_generated.` prefix
- Dashboards use Prometheus queries with container and pod filters
- Common query patterns are defined in jsonnet files for reuse
- The deployment uses in-memory storage (emptyDir with Memory medium)