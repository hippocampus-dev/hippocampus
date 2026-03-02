# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

tcp-proxy is a Go-based TCP proxy that supports connection pooling with advanced features like topology-aware routing, connection lifecycle management, and Prometheus metrics.

## Common Development Commands

### Development
- `make dev` - Run the proxy locally with watchexec for auto-reload on code changes
  - Default configuration: local=127.0.0.1:18888, remote=127.0.0.1:8888, monitor=127.0.0.1:8080
  - Includes connection pool settings: max-idle=100, min-idle=10, max-lifetime=10s

### Testing
- `go test` - Run all unit tests
- `go test -v` - Run tests with verbose output
- `go test -run TestConnection_Concurrency` - Run a specific test

### Benchmarking
- `make benchmark` - Run k6 performance tests (requires Docker Compose with k6 service)

### Building
- `go build -o tcp-proxy main.go` - Build the binary locally
- `docker build -t tcp-proxy .` - Build the Docker image

## Architecture

### Core Components

1. **Connection Pool** (`ConnectionPool`):
   - Manages TCP connections with configurable min/max idle connections
   - Supports FIFO/LIFO strategies for connection reuse
   - Implements connection health checking and lifecycle management
   - Features topology-aware routing for multi-zone deployments

2. **Connection Wrapper** (`Connection`):
   - Tracks connection metadata (creation time, last return time)
   - Implements health checking via syscall-level socket inspection
   - Supports graceful cancellation

3. **Proxy Server**:
   - Bidirectional TCP proxying with io.Copy
   - Graceful shutdown with configurable termination period
   - Lameduck period for client request draining
   - Semaphore-based connection limiting

### Key Features

- **Connection Pooling**: Maintains idle connections to reduce latency
- **Topology-Aware Routing**: Routes connections based on IP CIDR matching
- **Health Monitoring**: Prometheus metrics exposed on monitor endpoint
- **Jitter Support**: Adds randomization to timeouts to prevent thundering herd
- **Connection Lifecycle**: Max idle time and max lifetime configuration

## Configuration Flags

- `--local-address`: Local address to listen on
- `--remote-address`: Remote address to proxy to
- `--monitor-address`: Metrics endpoint address
- `--max-connections`: Maximum total connections
- `--max-idle-connections`: Maximum idle connections in pool
- `--min-idle-connections`: Minimum idle connections to maintain
- `--max-idle-time`: Maximum time a connection can stay idle
- `--max-lifetime`: Maximum lifetime of any connection
- `--jitter-percentage`: Jitter to apply to timeouts (0.0-1.0)
- `--topology-aware-routing`: Enable topology-based connection routing
- `--topologies`: Topology definitions (e.g., "zone1=10.0.0.0/24,zone2=10.1.0.0/24")
- `--own-ip`: Own IP for topology detection
