# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Grafana deployment manifest for Kubernetes using Kustomize. Dashboards are managed via grafana-manifest-controller using Dashboard custom resources with Jsonnet sources.

## Architecture

### Directory Structure
- `common.libsonnet` - Hard link to grafana-manifest-controller/ (gitignored)
- `jsonnetfile.json`, `jsonnetfile.lock.json` - Hard links to grafana-manifest-controller/ (gitignored)
- `Makefile` - Old method: generates JSON dashboards via jsonnet-builder
- `base/` - Base Kustomize resources
  - `jsonnet/dashboards/` - Jsonnet source files (kubernetes/, other/)
  - `dashboards/` - Generated JSON output from Makefile
  - `grafana_dashboard.yaml` - Dashboard custom resources with configMapRef
  - `kustomization.yaml` - Includes configMapGenerator for jsonnet files
- `overlays/dev/` - Development environment overlay with Istio integration
  - `jsonnet/` - Reserved for overlay-specific jsonnet (empty by default)

### Key Components
1. **Grafana Container** - Main Grafana instance with anonymous access enabled
2. **MCP Container** - MCP (Model Context Protocol) sidecar for Grafana integration
3. **Dashboard Management** - Dashboard CRs synced by grafana-manifest-controller

### Dashboard Categories
- **Kubernetes Dashboards**: cluster, node, namespace, workload, pod, volume, cronjob (folder: Kubernetes)
- **Other Dashboards**: overview, lighthouse (folder: Other)

### Hybrid Workflow

| Method | Command | Description |
|--------|---------|-------------|
| New (Controller) | `kubectl apply -k overlays/dev/` | Dashboard CRs with configMapRef, rendered by controller |
| Old (Makefile) | `make all` | Generates JSON files in `base/dashboards/` via jsonnet-builder |

**Note**: Shared files are managed via hard links:
- `common.libsonnet`, `jsonnetfile.json`, `jsonnetfile.lock.json` are hard links to `grafana-manifest-controller/`
- Editing either location updates both (same inode)
- `make link` or `make all` creates them automatically (gitignored)

### Important Patterns
- Jsonnet files use library-style imports: `import "common.libsonnet"`
- ConfigMap names follow `{category}-{name}.jsonnet` format (e.g., `kubernetes-cluster.jsonnet`)
- grafana-manifest-controller handles Jsonnet rendering and Grafana API sync
- The deployment uses in-memory storage (emptyDir with Memory medium)
