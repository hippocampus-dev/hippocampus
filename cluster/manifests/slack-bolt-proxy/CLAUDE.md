# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying the slack-bolt-proxy service using Kustomize. The slack-bolt-proxy acts as an intermediary between Slack's Socket Mode and an HTTP backend service, forwarding all Slack events with proper authentication.

## Common Development Commands

### Kustomize Commands
- `kubectl kustomize base/` - Build base manifests
- `kubectl kustomize overlays/dev/` - Build dev environment manifests
- `kubectl apply -k overlays/dev/` - Deploy to dev environment
- `kubectl diff -k overlays/dev/` - Preview changes before applying

### Validation
- `kubectl --dry-run=client -f <file>` - Validate individual manifest syntax
- `kustomize build overlays/dev/ | kubectl apply --dry-run=server -f -` - Server-side validation

## Architecture

### Directory Structure
- **base/**: Core Kubernetes resources
  - `deployment.yaml`: Main deployment with security context and OpenTelemetry configuration
  - `kustomization.yaml`: Base kustomization with image digests
  - `pod_disruption_budget.yaml`: PDB for high availability
  
- **overlays/dev/**: Dev environment customizations
  - `kustomization.yaml`: Main overlay configuration
  - `patches/`: Strategic merge patches for base resources
  - `peer_authentication.yaml`: Istio mTLS configuration
  - `secrets_from_vault.yaml`: Vault secret generator for Slack credentials
  - `sidecar.yaml`: Istio sidecar configuration
  - `telemetry.yaml`: Istio telemetry configuration

### Key Configurations

#### Security
- Non-root user (65532) with read-only filesystem
- All capabilities dropped, no privilege escalation
- RuntimeDefault seccomp profile
- Automount service account token disabled

#### High Availability
- 2 replicas with topology spread constraints across nodes and zones
- Rolling update strategy with maxSurge 25% and maxUnavailable 1
- PodDisruptionBudget allowing 1 unavailable pod

#### Observability
- OpenTelemetry traces exported to otel-agent
- Prometheus metrics exposed on port 8081
- Istio telemetry for request/response metrics

#### Secret Management
- Uses custom SecretsFromVault generator to fetch from HashiCorp Vault:
  - SLACK_APP_TOKEN
  - SLACK_BOT_TOKEN
  - SLACK_SIGNING_SECRET

### Environment-specific Settings
The dev overlay adds:
- Namespace: cortex-bot
- Backend target: cortex-bot.cortex-bot.svc.cluster.local:8080
- Istio sidecar injection with resource limits
- mTLS peer authentication in STRICT mode