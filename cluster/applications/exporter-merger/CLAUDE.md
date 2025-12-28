# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Exporter-merger is a lightweight HTTP server that merges Prometheus metrics from multiple exporters into a single endpoint. It has no external dependencies and is implemented using only the Go standard library.

## Common Development Commands

### Development
- `make dev` - Run the server locally
- `make all` - Format, lint, and test

### Build and Test
- `go build -o exporter-merger main.go` - Build the binary
- `CGO_ENABLED=0 go build -o exporter-merger main.go` - Build static binary
- `go run main.go` - Run locally

### Docker Build
- `docker build -t exporter-merger .` - Build the container image

## High-Level Architecture

### Core Components
1. **HTTP Server**: Listens on configurable address (default: `0.0.0.0:8080`)
2. **Metrics Handler**: Fetches and merges metrics from multiple exporters (`/metrics`)
3. **Health Check**: Provides `/healthz` endpoint

### Key Design Decisions
- **No External Dependencies**: Pure Go implementation without go.mod for simplicity
- **Parallel Fetching**: Concurrently fetches metrics from all configured exporters
- **Prometheus Format**: Parses and merges metrics in Prometheus text format
- **Distroless Container**: Minimal attack surface with non-root user (uid: 65532)
- **Graceful Shutdown**: Proper signal handling with configurable grace periods

### Configuration Flags
- `--address`: HTTP server listen address (ENV: ADDRESS)
- `--urls`: Space-separated list of exporter URLs to scrape (ENV: MERGER_URLS)
- `--termination-grace-period`: Time to wait for connections to close on shutdown (ENV: TERMINATION_GRACE_PERIOD)
- `--lameduck`: Period to reject new connections before shutdown (ENV: LAMEDUCK)
- `--http-keepalive`: Enable/disable HTTP keep-alive connections (ENV: HTTP_KEEPALIVE)

## Usage Example

```bash
exporter-merger --urls "http://localhost:9100/metrics http://localhost:9101/metrics"
```

Or using environment variables:

```bash
MERGER_URLS="http://localhost:9100/metrics http://localhost:9101/metrics" exporter-merger
```
