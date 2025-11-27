# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

events-logger is a Kubernetes controller that logs all Kubernetes events to stdout in JSON format. It runs as a deployment with leader election to ensure only one instance processes events at a time while maintaining high availability.

## Common Development Commands

### Development
- `make dev` - Runs the application in development mode using Skaffold with auto-rebuild and port forwarding
- `go mod tidy` - Clean up dependencies after adding/removing imports
- `go mod download` - Download all dependencies

### Building
- `go build -trimpath -o events-logger main.go` - Build the Go binary locally
- `CGO_ENABLED=0 GOOS=linux go build -trimpath -o events-logger main.go` - Build static Linux binary for containers
- Docker builds are handled automatically by Skaffold during development

### Testing
- `go test ./...` - Run all tests (Note: currently no tests implemented)
- `go vet ./...` - Run static analysis
- `golangci-lint run` - Run comprehensive linting (if golangci-lint is installed)

## High-Level Architecture

### Core Components

1. **Kubernetes Controller Pattern**:
   - Uses client-go's informer framework to watch Event resources (v1.Event)
   - Implements a workqueue with exponential backoff for reliable event processing
   - Processes events through AddFunc, UpdateFunc, and DeleteFunc handlers
   - Rate limiting prevents overwhelming during event bursts

2. **Leader Election Mechanism**:
   - Uses Kubernetes Lease resources in the kube-system namespace
   - Configuration: 60s lease duration, 15s renew deadline, 5s retry period
   - Ensures exactly one instance processes events despite running 2 replicas
   - Non-leaders stay ready to take over if the leader fails

3. **Event Processing Flow**:
   ```
   Kubernetes API → Informer → Work Queue → Process Worker → JSON to stdout
   ```
   - Configurable worker concurrency (CONCURRENCY env var, default: 1, production: 2)
   - Each event is marshaled to JSON and printed to stdout
   - Failed events are re-queued with exponential backoff

### Deployment Architecture

- **High Availability**: 2 replicas with PodDisruptionBudget (minAvailable: 1)
- **Security Hardening**:
  - Runs as non-root user (UID: 65532)
  - Read-only root filesystem with writable /tmp
  - All Linux capabilities dropped
  - No privilege escalation allowed
  - Distroless base image for minimal attack surface
- **Resource Management**:
  - GOMAXPROCS automatically set from CPU limits
  - GOMEMLIMIT automatically set from memory limits (90% of limit)
  - Prevents OOM kills and optimizes Go runtime performance

### RBAC Permissions

The service account requires:
- **Cluster-wide**: Watch all Event resources (`events.k8s.io/events`: get, list, watch)
- **Namespace-scoped**: Leader election in kube-system (`coordination.k8s.io/leases`: get, update, create)
- **Metrics access**: Read node metrics (`""/nodes/metrics`: get)

### Key Design Decisions

1. **JSON-only Output**: Structured logging for easy parsing by Fluentd/Elasticsearch
2. **In-Cluster Only**: Always uses in-cluster config, not designed for out-of-cluster use
3. **Stateless Design**: No persistent storage, events are processed and immediately logged
4. **Minimal Dependencies**: Only essential Kubernetes client libraries
5. **Graceful Shutdown**: Proper signal handling with 30-second termination grace period

## Code Structure

The entire application is in `main.go` with clear separation:
- `main()`: Sets up logging, client, and starts controller
- `run()`: Implements the controller logic with informer and workqueue
- `processNextWorkItem()`: Core event processing loop
- `handleEvent()`: Marshals events to JSON and outputs them

Environment variables are read directly without a config struct since there are only two settings (CONCURRENCY and leader election identity).