# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The ephemeral-container is a multi-purpose debugging and utility container designed for Kubernetes environments. It serves as a Swiss Army knife container packed with debugging, monitoring, and operational tools for cloud-native infrastructure.

## Common Development Commands

### Building the Container
```bash
# Build locally with Docker
docker build -t ephemeral-container .

# Build with buildx for multi-platform
docker buildx build --platform linux/amd64,linux/arm64 -t ephemeral-container .
```

### Running the Container
```bash
# Run interactively for debugging
docker run -it --rm ephemeral-container /bin/bash

# Run with host proc mounted (for process inspection)
docker run -it --rm -v /proc:/host/proc ephemeral-container /bin/bash

# Run armyknife echo server
docker run -it --rm -p 8080:8080 ephemeral-container armyknife echo-server
```

## High-Level Architecture

### Container Structure
- **Base**: Ubuntu 24.04 with non-root user (UID 65532)
- **User**: "admin" with sudo NOPASSWD privileges
- **Tools**: Pre-installed debugging, monitoring, and database clients
- **Custom Binaries**: Downloads armyknife, insight, and l7sniff from GitHub releases during build

### Key Components
1. **Network Tools**: tcpdump, tshark, curl, netcat, dnsutils
2. **Database Clients**: redis-tools, mysql-client, postgresql-client, cqlsh, memcached-tools
3. **Cloud Storage**: AWS CLI v2, Google Cloud CLI, MinIO client, DuckDB with S3 support
4. **Observability**: logcli (Loki), mimirtool (Mimir), td-agent4 (Fluentd)
5. **Custom Tools**:
   - `armyknife`: Multi-purpose CLI utility with various subcommands
   - `insight`: eBPF-based process monitoring tool
   - `l7sniff`: Layer 7 network traffic analyzer

### Pre-configured Environment
- MinIO endpoint and credentials
- AWS credentials
- Loki and Mimir endpoints
- Memcached servers configuration
- DuckDB with MinIO S3 secrets

### Special Features
- Modified procps tools to read from `/host/proc` for host process inspection
- Pre-configured DuckDB extensions for S3/Parquet operations
- Jupyter notebook support for interactive analysis

## Development Workflow

1. The container is built automatically via GitHub Actions when changes are pushed
2. Binary dependencies (armyknife, insight, l7sniff) are built separately in their respective projects
3. The container downloads these binaries from GitHub releases during build
4. Published to ghcr.io/hippocampusai/ephemeral-container

### Testing Changes
```bash
# Test the Dockerfile build locally
docker build -t test-ephemeral .

# Verify tools are installed correctly
docker run --rm test-ephemeral armyknife --version
docker run --rm test-ephemeral insight --version
docker run --rm test-ephemeral which tcpdump
```

### Common Use Cases
1. **Kubernetes debugging pod**: Deploy as ephemeral container for troubleshooting
2. **Network analysis**: Use tcpdump/tshark/l7sniff for packet inspection
3. **Database connectivity testing**: Test connections to various databases
4. **Cloud storage operations**: Interact with S3-compatible storage systems
5. **Registry maintenance**: Clean up old Docker images in registries