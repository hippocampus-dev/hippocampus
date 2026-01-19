# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The `exactly-one-pod-hook` is a Kubernetes admission webhook that ensures only one pod of a given type runs at a time using distributed locking. It supports both Redis (Redlock algorithm) and etcd as lock backends.

## Common Development Commands

### Primary Development
- `make dev` - Runs Skaffold with port-forwarding for local Kubernetes development (monitors code changes and auto-redeploys)

### Go Development
- `go mod tidy` - Clean up module dependencies
- `go build` - Build the binary locally
- `go fmt ./...` - Format all Go code
- `go vet ./...` - Run static analysis
- `go test ./...` - Run tests (when available)

### Docker Build
- The application uses a multi-stage Dockerfile with distroless base image
- CGO is disabled for static binary compilation
- Binary name: `exactly-one-pod-hook`

## High-Level Architecture

### Command Structure
The application uses Cobra CLI with three commands:
1. **webhook** (default) - Runs the admission webhook server
2. **sidecar** - Runs as a sidecar container to maintain locks
3. **root** - Global flags and lock configuration

### Core Components
- **`cmd/`** - CLI command definitions
  - `root.go` - Global flags and lock mode configuration
  - `webhook.go` - Webhook server implementation
  - `sidecar.go` - Sidecar container logic
- **`internal/lock/`** - Lock abstraction supporting Redis and etcd
- **`pkg/webhook/`** - Admission webhook logic and pod mutation
- **`pkg/sidecar/`** - Sidecar implementation for lock maintenance

### How It Works
1. Pods annotated with `exactly-one-pod-hook.kaidotio.github.io/exactly-one-pod: "true"` trigger the webhook
2. The webhook attempts to acquire a distributed lock using the configured backend
3. If successful, it injects a sidecar container that maintains the lock
4. If the lock is already held, pod creation is denied
5. The sidecar releases the lock when the pod terminates

### Lock Backends
- **Redis**: Uses Redlock algorithm for distributed locking
- **etcd**: Uses etcd's built-in distributed locking
- Configured via `--lock-mode` flag (redis or etcd)

### Kubernetes Integration
- Deployed as a MutatingAdmissionWebhook
- Uses cert-manager for TLS certificates
- Includes service account and RBAC configurations
- Supports metrics and health endpoints

### Development Workflow
1. Use `make dev` to start Skaffold development loop
2. Skaffold watches for changes and automatically rebuilds/redeploys
3. Port-forwarding is set up automatically for local access
4. Kustomize is used for manifest management in `skaffold/` directory