# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A simple URL shortener service built in Go that uses an external HTTP Key-Value Store (http-kvs) for persistence. The service provides URL shortening functionality with SHA256-based hash generation and is designed for Kubernetes deployment.

## Common Development Commands

- `make dev` - Run the application with hot-reload using watchexec (monitors file changes and restarts automatically)
- `go run *.go` - Run the application directly
- `docker build -t url-shortener .` - Build the Docker image

### Command-line flags
- `-address` - Listen address (default: "0.0.0.0:8080")
- `-http-kvs-url` - Backend KVS URL (default: "https://http-kvs.minikube.127.0.0.1.nip.io")
- `-termination-grace-period-seconds` - Graceful shutdown period (default: 10)
- `-lameduck` - Pre-shutdown delay in seconds (default: 1)
- `-http-keepalive` - Enable HTTP keep-alive (default: true)

## High-Level Architecture

### Service Architecture
This is a single-file Go application (`main.go`) that acts as a stateless proxy to an http-kvs backend:

1. **POST /** - Creates shortened URLs
   - Accepts URL in request body
   - Generates SHA256 hash of the URL
   - Stores mapping in http-kvs at `/url_shortener/{hash}`
   - Returns the hash to the client

2. **GET /{hash}** - Redirects to original URL
   - Retrieves URL from http-kvs
   - Returns 301 redirect to the original URL

3. **GET /healthz** - Health check endpoint for Kubernetes probes

### Key Design Patterns
- **Stateless Design**: All URL mappings stored in external http-kvs service
- **Graceful Shutdown**: Handles SIGTERM with configurable grace period and lameduck delay
- **No External Dependencies**: Uses only Go standard library
- **Security-First Container**: Runs as non-root user in distroless image
- **Kubernetes-Native**: Built-in health checks and graceful termination

### Docker Build Strategy
Multi-stage build:
1. Builder stage compiles static binary with optimizations (`-trimpath`, `-ldflags="-s -w"`)
2. Runtime stage uses distroless/static:nonroot for minimal attack surface

### Dependencies
- Requires running http-kvs service for storage backend
- Development requires watchexec for hot-reload functionality