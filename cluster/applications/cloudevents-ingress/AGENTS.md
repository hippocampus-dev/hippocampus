# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CloudEvents Ingress is a Knative Serving application that receives incoming HTTP requests and converts them to CloudEvents, then forwards them to a configurable Knative Broker. It serves as an ingress gateway for external systems that need to publish events into the Knative Eventing ecosystem without implementing the CloudEvents protocol.

## Common Development Commands

### Primary Development
- `make dev` - Runs with watchexec for auto-reload development
- `make all` - Run full checks: format, lint, tidy, test

### Building and Dependencies
- `go mod tidy` - Tidy Go dependencies
- `go test -race ./...` - Run tests
- Docker multi-stage build creates a distroless container with the `cloudevents-ingress` binary

## High-Level Architecture

### Request Flow
1. External HTTP POST requests arrive at the service
2. The service reads the request body and optional CloudEvents headers (`Ce-Type`, `Ce-Source`, `Ce-Subject`)
3. A CloudEvent is constructed with a new UUID, the provided or default type/source, and the request body as data
4. The CloudEvent is forwarded to the configured broker URL
5. The service returns HTTP 202 Accepted on success, or HTTP 502 Bad Gateway if delivery fails

### Configuration

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--sink-url` | `K_SINK` | (required) | Sink URL to send CloudEvents to (injected by Knative SinkBinding) |
| `--oidc-token-path` | `OIDC_TOKEN_PATH` | | Path to OIDC token file for broker authentication |
| `--default-type` | `DEFAULT_TYPE` | `cloudevents-ingress.event` | Default CloudEvent type |
| `--default-source` | `DEFAULT_SOURCE` | `cloudevents-ingress` | Default CloudEvent source |

### Deployment Structure
- **Knative Service** with scale-to-zero (0-10 replicas)
- Exposed via Istio Gateway for external access
- Deployed as a utility of memory-bank (manifests in `cluster/manifests/utilities/cloudevents-ingress/`, consumer in `cluster/manifests/memory-bank/overlays/dev/cloudevents-ingress/`)
- Shares memory-bank's namespace and ArgoCD Application
- Uses **SinkBinding** to inject `K_SINK` from the Kafka Broker (no hardcoded broker URL)
- Uses **projected ServiceAccount token** for OIDC authentication to the broker
