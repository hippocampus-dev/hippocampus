# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains the Docker packaging configuration for configurable-http-proxy, a component of the JupyterHub ecosystem within the Hippocampus platform. It packages the official npm package with additional Redis backend support for distributed proxy table storage.

## Common Development Commands

### Building the Docker Image
```bash
# Build locally (from this directory)
docker build -t configurable-http-proxy:local .

# The CI/CD pipeline automatically builds and pushes images when changes are made
```

### Updating Dependencies
- To update configurable-http-proxy version: Modify the base image tag in the Dockerfile (currently `4.6.1`)
- To update Redis backend version: Modify the npm install command in the Dockerfile (currently `0.1.6`)

## High-Level Architecture

### Purpose
Configurable-http-proxy serves as the reverse proxy for JupyterHub, routing HTTP requests between:
- The JupyterHub hub (control plane)
- Individual user notebook servers
- Other JupyterHub services

### Key Configuration
- **Port 8080**: Main HTTP proxy port for user traffic
- **Port 8081**: API port for dynamic route updates from JupyterHub
- **Port 8082**: Metrics endpoint for monitoring
- **Redis Backend**: Enables distributed proxy table storage for high availability

### Integration Points
1. **JupyterHub**: Receives routing table updates via the API port
2. **User Servers**: Proxies requests to dynamically created notebook servers
3. **Redis**: Stores routing table for persistence and multi-instance deployments
4. **Kubernetes**: Deployed as part of the JupyterHub stack with security constraints

### CI/CD Workflow
Changes to the Dockerfile trigger:
1. Docker image build and push to GitHub Container Registry
2. Automatic PR creation to update Kubernetes manifests with new image digest
3. Deployment after PR merge

## Important Notes
- This is a packaging repository only - the actual proxy logic is in the upstream npm packages
- The proxy runs as non-root user (65534) with a read-only filesystem for security
- Health checks are available at `/_chp_healthz` endpoint
- The default proxy target is configured to point to the JupyterHub hub service