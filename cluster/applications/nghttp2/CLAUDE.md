# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains a Dockerfile for building nghttp2 - a C library implementation of HTTP/2 protocol and its applications. The container provides HTTP/2 client, server, proxy, and benchmarking tools used within the Hippocampus cluster.

## Common Development Commands

### Building the Docker Image
```bash
# Basic build
docker build -t nghttp2 .

# Build with BuildKit (recommended for better caching)
DOCKER_BUILDKIT=1 docker build -t nghttp2 .

# Build with a specific tag for registry
docker build -t ghcr.io/hippocampus-dev/hippocampus/nghttp2:latest .

# Build with a different nghttp2 version
docker build --build-arg NGHTTP2_VERSION=v1.62.0 -t nghttp2 .
```

### Testing the Built Image
```bash
# Run nghttp client
docker run --rm nghttp2 nghttp --version

# Run nghttpd server
docker run --rm -p 8080:8080 nghttp2 nghttpd --no-tls 8080

# Run h2load benchmarking tool
docker run --rm nghttp2 h2load --help
```

## High-Level Architecture

### Build Process
The Dockerfile uses a multi-stage build approach:
1. **Builder stage**: Compiles nghttp2 from source with static linking of dependencies (OpenSSL, libev, c-ares, jemalloc, zlib)
2. **Final stage**: Creates a minimal Debian-based image containing only the necessary binaries

### Included Tools
- `nghttp`: HTTP/2 client for testing and debugging
- `nghttpd`: HTTP/2 server
- `nghttpx`: HTTP/2 proxy with various backend protocol support
- `h2load`: HTTP/2 benchmarking tool for load testing

### Design Decisions
- Static compilation to minimize runtime dependencies
- Multi-stage build to reduce final image size
- Uses Debian bookworm-slim for compatibility
- Configurable nghttp2 version via build argument (default: v1.61.0)
- Optimized with `install-strip` to remove debug symbols