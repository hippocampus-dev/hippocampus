# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Alerthandler is a Knative Serving webhook receiver that processes alerts from Prometheus Alertmanager. It receives webhook requests, parses alert payloads, and dispatches them to appropriate handlers based on the alert name. Currently, it supports automatic pod remediation for memory-related alerts.

## Common Development Commands

### Primary Development
- `make dev` - Runs skaffold dev with port-forwarding for hot-reload development

### Building and Dependencies
- `make all` - Runs format, lint, tidy, and tests
- `make fmt` - Formats Go code using gofmt and goimports
- `make lint` - Runs go vet for static analysis
- `make tidy` - Tidies Go dependencies
- `make test` - Runs tests with race detection and benchmarks

### Viewing Logs
```bash
# Development environment
kubectl logs -n skaffold-alerthandler -l app.kubernetes.io/name=alerthandler

# Production environment
kubectl logs -n <namespace> -l app.kubernetes.io/name=alerthandler
```

## High-Level Architecture

### Alert Flow
1. Prometheus Alertmanager sends webhook requests to this service
2. The service parses the AlertManagerRequest payload
3. Alerts are dispatched to handlers based on the alertname label
4. Handlers execute remediation actions using the Kubernetes API

### Handler Pattern
- **Interface**: All handlers implement the `Handler` interface with a `Call` method
- **Dispatch**: The `Dispatch` function routes alerts to appropriate handlers based on alertname
- **NotFoundError**: Alerts without matching handlers return a NotFoundError (logged but not treated as failure)

### Available Handlers
- **RunOutContainerMemoryHandler**: Handles `RunOutContainerMemory` alerts by:
  1. Looking up PodDisruptionBudgets in the affected namespace
  2. Waiting for disruption budget to allow deletions
  3. Deleting the affected pod to trigger a restart

### Deployment Configuration
- Runs as a Knative Service with cluster-local visibility
- Configured with 0-10 replica autoscaling based on concurrency
- Uses strict security context:
  - Non-root user (65532)
  - Read-only root filesystem
  - No privilege escalation
  - All capabilities dropped
- Requires in-cluster Kubernetes API access for pod management

### Key Design Decisions
1. **Kubernetes Client**: Uses in-cluster config for Kubernetes API access
2. **Handler Dispatch**: Pattern allows easy addition of new alert handlers
3. **PDB Awareness**: Respects PodDisruptionBudgets before deleting pods
4. **Graceful Shutdown**: Implements proper signal handling with 10-second timeout
