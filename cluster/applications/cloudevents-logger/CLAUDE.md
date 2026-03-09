# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CloudEvents Logger is a simple Knative Eventing service that receives and logs CloudEvents. It serves as an event sink for debugging event flows in Knative/Kubernetes environments, acknowledging all received events and printing them to stdout.

## Common Development Commands

### Primary Development
- `make dev` - Runs skaffold dev with port-forwarding for hot-reload development

### Building and Dependencies
- `go mod tidy` - Tidy Go dependencies
- `go test` - Run tests (no test files currently exist)
- Docker multi-stage build creates a distroless container with the `cloudevents-logger` binary

### Viewing Logs
```bash
# Development environment
kubectl logs -n skaffold-cloudevents-logger -l app.kubernetes.io/name=cloudevents-logger

# Production environment  
kubectl logs -n <namespace> -l app.kubernetes.io/name=cloudevents-logger
```

## High-Level Architecture

### Event Flow
1. CloudEvents are sent to the Knative Broker named `cloudevents-logger`
2. A Trigger routes all events from the broker to this service
3. The service logs the event to stdout and returns an acknowledgment
4. In development, a PingSource sends test events every minute

### Deployment Structure
- **Production** (`/manifests/`): Deploys with 0-10 replica autoscaling
- **Development** (`/skaffold/`): Deploys to `skaffold-cloudevents-logger` namespace with 0-1 replica autoscaling and includes a PingSource for testing

### Security Configuration
- Runs as non-root user (65532)
- Read-only root filesystem
- No privilege escalation allowed
- All capabilities dropped
- RuntimeDefault seccomp profile

## Key Implementation Details

The service is implemented in `main.go` with a simple handler that:
1. Receives CloudEvents via the CloudEvents SDK for Go
2. Logs the entire event to stdout
3. Returns `cloudevents.ResultACK` for all events

No event processing or transformation occurs - this is purely a logging sink for debugging purposes.