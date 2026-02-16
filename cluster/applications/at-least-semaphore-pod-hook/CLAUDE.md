# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`at-least-semaphore-pod-hook` is a Kubernetes admission webhook that ensures a minimum number of pods (at least N) can run concurrently for a given deployment or resource type. It uses distributed locking (Redis/Redlock or etcd) to manage a semaphore-based queue system.

## Common Development Commands

### Local Development
- `make dev` - Runs Skaffold in development mode with port forwarding for hot-reload development
- `skaffold dev --port-forward` - Same as above, direct Skaffold command

### Building
- `go build` - Build the binary locally
- `docker build -t at-least-semaphore-pod-hook .` - Build Docker image

### Testing
- `go test ./...` - Run tests (note: no tests currently exist)

### Go Module Management
- `go mod tidy` - Clean up dependencies
- `go mod download` - Download dependencies

## High-Level Architecture

### Components

1. **Webhook Server** (`cmd/webhook.go`, `pkg/webhook/`)
   - Kubernetes admission webhook that intercepts pod creation
   - Manages distributed semaphore using Redis or etcd
   - Injects sidecar containers into pods that require semaphore management
   - Handles queue initialization and management

2. **Sidecar** (`cmd/sidecar.go`, `pkg/sidecar/`)
   - Runs alongside application containers
   - Handles graceful shutdown by releasing semaphore on pod termination
   - Moves items from inflight set back to queue on SIGTERM

3. **Lock System** (`internal/lock/`)
   - Abstraction over Redis (Redlock) and etcd distributed locking
   - Ensures atomic operations on semaphore queue

### How It Works

1. Pods annotated with `at-least-semaphore-pod-hook.kaidotio.github.io/at-least-semaphore-pod: "true"` trigger the webhook
2. The webhook checks/initializes a queue with the specified length (minimum pod count)
3. When a pod is created, it attempts to pop from the queue
4. If successful, the pod gets a sidecar container that will release the semaphore on termination
5. The sidecar listens for SIGTERM and moves the semaphore from "inflight" back to "queue"

### Key Annotations

Pods must have these annotations:
- `at-least-semaphore-pod-hook.kaidotio.github.io/at-least-semaphore-pod: "true"` - Enable the webhook
- `at-least-semaphore-pod-hook.kaidotio.github.io/key: "resource-name"` - Semaphore key name
- `at-least-semaphore-pod-hook.kaidotio.github.io/length: "3"` - Minimum number of pods
- `at-least-semaphore-pod-hook.kaidotio.github.io/expiration: "60"` - Lock expiration in seconds

### Command Line Arguments

#### Common Flags
- `--lock-mode` - Lock backend: "redlock" or "etcd"
- `--redis-addresses` - Redis server addresses for Redlock
- `--etcd-addresses` - etcd server addresses
- `--queue-redis-address` - Redis server for queue management

#### Webhook-specific Flags
- `--host` - Webhook server host
- `--port` - Webhook server port (default: 9443)
- `--certDir` - TLS certificate directory
- `--sidecar-image` - Docker image for sidecar containers
- `--enable-sidecar-containers` - Use Kubernetes sidecar containers feature

#### Sidecar-specific Flags
- `--termination-grace-period-seconds` - Grace period for shutdown

### Deployment

The webhook is typically deployed using:
1. Kubernetes Deployment with TLS certificates (cert-manager)
2. MutatingWebhookConfiguration to intercept pod creation
3. Service to expose the webhook
4. Redis or etcd cluster for distributed locking