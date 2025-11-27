# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Anonymous-proxy is a lightweight HTTP proxy server that forwards OpenID Connect JWKS (JSON Web Key Set) requests to the Kubernetes API server. It acts as an authentication bridge, allowing anonymous access to specific Kubernetes API endpoints while handling authentication using its service account credentials.

## Common Development Commands

### Build and Test
- `go build -o anonymous-proxy main.go` - Build the binary
- `CGO_ENABLED=0 go build -o anonymous-proxy main.go` - Build static binary (as used in Dockerfile)
- `go run main.go` - Run locally (requires Kubernetes service account setup)

### Docker Build
- `docker build -t anonymous-proxy .` - Build the container image

## High-Level Architecture

### Core Components
1. **HTTP Server**: Listens on configurable address (default: `0.0.0.0:8080`)
2. **Proxy Handler**: Forwards `/openid/v1/jwks` requests to Kubernetes API server
3. **Health Check**: Provides `/healthz` endpoint for Kubernetes readiness/liveness probes
4. **Service Account Integration**: Reads token and CA certificate from standard Kubernetes mount paths

### Key Design Decisions
- **No External Dependencies**: Pure Go implementation without go.mod for simplicity
- **Distroless Container**: Minimal attack surface with non-root user (uid: 10001)
- **Graceful Shutdown**: Proper signal handling with configurable grace periods
- **TLS Client Configuration**: Uses Kubernetes CA certificate for secure API communication

### Request Flow
1. Client makes anonymous request to `/openid/v1/jwks`
2. Proxy reads service account token from `/var/run/secrets/kubernetes.io/serviceaccount/token`
3. Proxy configures TLS client with CA from `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`
4. Proxy forwards request to `https://kubernetes.default.svc/openid/v1/jwks` with Bearer token
5. Response is returned to client unchanged

### Configuration Flags
- `--address`: HTTP server listen address
- `--termination-grace-period-seconds`: Time to wait for connections to close on shutdown
- `--lameduck`: Period to reject new connections before shutdown
- `--http-keepalive`: Enable/disable HTTP keep-alive connections