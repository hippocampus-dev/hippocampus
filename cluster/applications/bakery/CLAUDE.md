# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build Commands
```bash
# Build binary locally (no external dependencies)
CGO_ENABLED=0 go build -trimpath -o bakery -ldflags="-s -w" main.go

# Build Docker image
docker build -t bakery .

# Run locally with defaults
./bakery

# Run with custom configuration
./bakery -address=0.0.0.0:9090 -termination-grace-period-seconds=30 -lameduck=5
```

### Command-line Flags
- `-address`: HTTP listen address (default: "0.0.0.0:8080")
- `-termination-grace-period-seconds`: Graceful shutdown timeout in seconds (default: 10)
- `-lameduck`: Seconds to stop accepting new requests before shutdown (default: 1)
- `-http-keepalive`: Enable HTTP keep-alive connections (default: true)

## High-Level Architecture

The bakery service is a minimal Go HTTP server designed for handling OAuth callbacks in Kubernetes environments. It validates cookies and redirects users with the cookie value appended as query parameters.

### Key Components
1. **Cookie Callback Handler** (`/callback`): Validates a cookie named "c" and redirects to URLs with cookie value and expiration time
2. **Health Check** (`/healthz`): Simple endpoint returning 200 OK for Kubernetes liveness/readiness probes
3. **Graceful Shutdown**: Handles SIGTERM signals with configurable lameduck and termination grace periods

### Design Patterns
- **Zero Dependencies**: Uses only Go standard library, no go.mod file needed
- **Kubernetes-Native**: Graceful shutdown with lameduck period for pod termination
- **Security-First**: Runs as non-root user (65532) in distroless container
- **Minimal Surface**: Only two endpoints, focused single responsibility

### Docker Build Strategy
- Multi-stage build with Go 1.23.5 Alpine for compilation
- Final image uses gcr.io/distroless/static:nonroot
- Binary compiled with CGO_ENABLED=0 for static linking
- Security flags: -trimpath and -ldflags="-s -w" for smaller, cleaner binaries