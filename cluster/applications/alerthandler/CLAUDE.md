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

### Dispatcher Pattern
- **Dispatcher**: Central struct that holds dependencies (Kubernetes client, `*github.Client`) and routes alerts
  - `NewDispatcher(kubernetes, gitHubClient)` - Constructor with dependency injection
  - `Handle(*AlertManagerRequest)` - Entry point that dispatches and calls the appropriate handler
  - `Dispatch(*AlertManagerRequest)` - Routes alerts to handlers based on severity and alertname
- **Handler Interface**: All handlers implement the `Handler` interface with a `Call` method
- **NotFoundError**: Alerts without matching handlers return a NotFoundError (logged but not treated as failure)

### Available Handlers
- **CriticalAlertHandler**: Handles alerts with `severity=critical` by:
  - Holds `*github.Client` (go-github v68)
  - `NewCriticalAlertHandler(client)` - Constructor
  - Requires `repository` label on each alert (format: `owner/repo`)
  - Creates GitHub issue with alert details, labels, and annotations
  - Applies `alert` and `critical` labels to the created issue
  - Testing: Use `httptest.Server` to mock GitHub API, set `client.BaseURL` to server URL

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
- Requires `GITHUB_TOKEN` environment variable for GitHub issue creation (CriticalAlertHandler)

### Key Design Decisions
1. **Dependency Injection**: Dispatcher struct encapsulates dependencies (Kubernetes client, `*github.Client`)
2. **Direct go-github Usage**: CriticalAlertHandler uses `*github.Client` directly without wrapper; tests use `httptest.Server` with custom `BaseURL`
3. **Kubernetes Client**: Uses in-cluster config for Kubernetes API access
4. **Handler Dispatch**: Severity-based routing takes precedence over alertname-based routing (critical severity alerts are always handled by CriticalAlertHandler)
5. **GitHub Token**: Uses `GITHUB_TOKEN` environment variable with `github.NewClient(nil).WithAuthToken()`
6. **Required Labels**: CriticalAlertHandler requires `repository` label (format: `owner/repo`) on each alert
7. **PDB Awareness**: Respects PodDisruptionBudgets before deleting pods
8. **Graceful Shutdown**: Implements proper signal handling with 10-second timeout
