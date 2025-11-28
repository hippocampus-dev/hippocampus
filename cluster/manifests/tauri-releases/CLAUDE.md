# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the tauri-releases service, which serves as a static file server for Tauri application (`taurin`) releases. The service uses nginx to distribute application updates to end users.

## Common Development Commands

### Local Testing
```bash
# From the applications directory
cd ../../applications/tauri-releases
mkdir -p releases
docker build -t tauri-releases .
docker run -p 8080:8080 -v $(pwd)/releases:/usr/share/nginx/html:ro tauri-releases

# Test endpoints
curl http://localhost:8080/healthz
curl http://localhost:8080/latest.json
```

### Kubernetes Deployment Updates
```bash
# Update image digest in kustomization.yaml
cd base
kustomize edit set image ghcr.io/kaidotio/hippocampus/tauri-releases=ghcr.io/kaidotio/hippocampus/tauri-releases@sha256:NEW_DIGEST
```

## High-Level Architecture

### GitOps Workflow
1. GitHub Actions workflow (`/.github/workflows/00_tauri-releases.yaml`) runs on:
   - Changes to the application code or manifests
   - Monthly schedule
   - Manual dispatch

2. Build Process:
   - Downloads latest `taurin` release files from GitHub
   - Updates `latest.json` with the correct base URL
   - Builds Docker image with release files
   - Creates PR with updated image digest in `base/kustomization.yaml`

3. ArgoCD picks up the changes and deploys to Kubernetes

### Kubernetes Resources
- **Deployment**: Runs nginx container with static files
- **Service**: Exposes the deployment on port 80
- **HPA**: Auto-scales based on CPU/memory (1-5 replicas)
- **PDB**: Ensures at least 1 pod is always available

### Key Configuration Details
- Container runs as non-root user (UID 65532)
- nginx listens on port 8080 internally
- Files served with `Cache-Control: max-age=0, must-revalidate`
- Gzip compression enabled
- Health check endpoint at `/healthz`
- Tauri apps check `https://tauri-releases.kaidotio.dev/latest.json` for updates