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
   - Sends periodic heartbeats to Redis Sorted Set (score=timestamp) to track liveness
   - On SIGTERM, moves item from inflight Sorted Set back to queue

3. **Lock System** (`internal/lock/`)
   - Abstraction over Redis (Redlock) and etcd distributed locking
   - Ensures atomic operations on semaphore queue

### How It Works

1. Pods annotated with `at-least-semaphore-pod-hook.kaidotio.github.io/at-least-semaphore-pod: "true"` trigger the webhook
2. The webhook acquires a distributed lock, then reclaims stale entries from the inflight Sorted Set (heartbeat score older than threshold) back to the queue
3. The webhook refills the queue with UUID tokens when total capacity (queue + inflight) drops below the desired length
4. The webhook releases the lock, then pops a token from the queue; if successful, the token is added to the inflight Sorted Set with the current timestamp as score
5. The pod gets a sidecar container that periodically heartbeats to the inflight Sorted Set (updating the score) and passes the token value as an argument
6. On SIGTERM, the sidecar moves its token from the inflight Sorted Set back to the queue

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
- `--heartbeat-interval-seconds` - Interval between sidecar heartbeats (default: 30)
- `--stale-threshold-seconds` - Seconds after which an inflight entry is considered stale (default: 120)

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
