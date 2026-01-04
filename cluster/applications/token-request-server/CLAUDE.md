# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

token-request-server is a Kubernetes API proxy service that provides RESTful endpoints for performing CRUD operations on Kubernetes resources. It acts as a simplified interface to the Kubernetes API, using dynamic clients to handle any resource type.

## Common Development Commands

### Development
- `make dev` - Run the service with Skaffold in development mode with port forwarding
- `skaffold dev --port-forward` - Same as above, runs the service in Kubernetes with hot-reload

### Build
- `go build -o token-request-server main.go` - Build the binary locally
- `docker build -t token-request-server .` - Build the Docker image

### Testing
- `go test ./...` - Run all tests (Note: currently no test files exist)
- `go test -v ./internal/...` - Run tests for internal packages with verbose output

### Linting and Formatting
- `go fmt ./...` - Format all Go files
- `go vet ./...` - Run Go vet for static analysis
- `golangci-lint run` - Run comprehensive linting (if golangci-lint is installed)

### Dependencies
- `go mod tidy` - Clean up and download dependencies
- `go mod verify` - Verify dependencies

## High-Level Architecture

### API Routes

The service exposes the following RESTful endpoints:

1. **Generic CRUD Operations**:
   - `POST /{namespace}/{group}/{version}/{kind}` - Create a resource
   - `GET /{namespace}/{group}/{version}/{kind}/{name}` - Read a specific resource
   - `PATCH /{namespace}/{group}/{version}/{kind}/{name}` - Update a resource
   - `DELETE /{namespace}/{group}/{version}/{kind}/{name}` - Delete a resource
   - `GET /{namespace}/{group}/{version}/{kind}` - List resources of a type

2. **Specialized Operations**:
   - `GET /` - List all namespaces
   - `POST /{namespace}/batch/v1/job/{name}/from/cronjob/{from}` - Create a Job from a CronJob
   - `POST /token` - Create a Kubernetes ServiceAccount token using the TokenRequest API

3. **Service Endpoints**:
   - `GET /healthz` - Health check endpoint
   - `GET /metrics` - Prometheus metrics endpoint
   - `GET /debug/pprof/*` - Profiling endpoints (only in debug mode)

### Key Components

- **main.go**: Entry point that sets up:
  - OpenTelemetry tracing with OTLP exporter
  - Pyroscope continuous profiling
  - Prometheus metrics collection
  - Audit logging middleware (when --audit-log-path is specified)
  - Graceful shutdown handling
  - In-cluster Kubernetes client configuration

- **internal/myhttp/router.go**: Custom HTTP router with middleware that provides:
  - Request tracing with OpenTelemetry
  - Automatic panic recovery
  - Request duration metrics
  - Structured logging with trace/span IDs
  - Support for custom middleware injection via Use() method

- **POST /token endpoint** (in main.go): TokenRequest API handler that:
    - Creates ServiceAccount tokens using the Kubernetes TokenRequest API
    - Accepts optional `audiences` field in JSON request body (array of strings); if omitted, uses Kubernetes default audience
    - Accepts optional `expirationSeconds` in JSON request body (aligned with Kubernetes TokenRequest API)
    - Validates expiration bounds per Kubernetes spec: minimum 600 seconds (10 minutes), maximum 1<<32 seconds (~136 years)
    - Default token expiration: 3600 seconds (1 hour) when not specified
    - Returns JSON response with token and expirationTimestamp (RFC3339Micro format)
    - Example request body: `{"audiences": ["https://example.com"], "expirationSeconds": 7200}` for 2-hour token with custom audience

- **Audit logging** (in main.go): Middleware that records:
  - HTTP request details (method, path, headers, body, remote address)
  - HTTP response details (status code, duration)
  - OpenTelemetry trace and span IDs
  - All entries written to file in JSON format
  - Sensitive headers are redacted (Authorization, Cookie, etc.)
  - Request body limited to 10KB

### Authentication & Authorization

The service runs with a ServiceAccount and uses in-cluster configuration to authenticate with the Kubernetes API. It inherits the permissions of its ServiceAccount, which should be configured via RBAC.

### Deployment

- Runs on Kubernetes with 2 replicas by default
- Uses distroless base image for security
- Configured with security best practices (non-root user, read-only filesystem, no capabilities)
- Supports horizontal pod autoscaling and pod disruption budgets
- Uses Kustomize for manifest management
