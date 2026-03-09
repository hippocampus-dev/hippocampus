# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a custom k6 container build for the Hippocampus platform. k6 is a load testing tool, and this application provides a k6 runtime with custom extensions for testing other services in the cluster.

## Architecture

This application consists of a single Dockerfile that:
1. Uses `grafana/xk6:0.12.2` to build k6 v0.53.0
2. Includes two extensions:
   - `xk6-dashboard` - Provides real-time test result dashboards
   - `xk6-sql` - Enables SQL database load testing
3. Creates a minimal distroless container image running as non-root user

## Common Development Commands

### Building the Container
```bash
docker build -t k6:local .
```

### Running k6 Tests
```bash
# Run a test script
docker run --rm k6:local run script.js

# Run with dashboard
docker run --rm -p 5665:5665 k6:local run --out web-dashboard script.js
```

## Usage Patterns

k6 is used throughout the Hippocampus cluster for load testing. Example test scripts can be found in:
- `/opt/hippocampus/cluster/applications/proxy-wasm/k6/` - Tests for WebAssembly filters
- `/opt/hippocampus/cluster/applications/tcp-proxy/k6/` - TCP proxy connection tests

## Deployment

k6 is deployed via the k6-operator in Kubernetes, which allows running distributed load tests. The operator manifests are located at `/opt/hippocampus/cluster/manifests/k6-operator/`.

Unlike most other applications in the cluster, k6 does not have a local Makefile or development setup - it's purely container-based.