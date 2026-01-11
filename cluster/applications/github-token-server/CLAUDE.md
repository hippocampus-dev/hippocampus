# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The github-token-server is a Kubernetes-native microservice that generates GitHub access tokens for GitHub Actions workflows. It authenticates as a GitHub App using JWT tokens and provides REST endpoints to generate scoped access tokens for users, organizations, and specific repositories.

## Common Development Commands

### Development
- `make dev` - Runs the service in development mode with auto-reload using watchexec. This automatically restarts the service on file changes and includes a hardcoded client ID for local testing.

### Building
- `go build` - Build the binary
- `docker build -t github-token-server .` - Build the Docker container

### Testing
- `go test ./...` - Run all tests

### Dependencies
- `go mod tidy` - Clean up dependencies

## High-Level Architecture

### Service Endpoints
- `/healthz` - Health check endpoint
- `/metrics` - Prometheus metrics endpoint
- `/users/{username}/access_tokens` - Generate tokens for user installations
- `/orgs/{org}/access_tokens` - Generate tokens for organization installations
- `/repos/{owner}/{repo}/access_tokens` - Generate repository-scoped tokens

Query parameter `permissions` accepts `reader` or `writer` profiles.

### Key Design Patterns
1. **GitHub App Authentication**: Uses JWT tokens signed with the app's private key
2. **Observability-First**: Integrated OpenTelemetry tracing, Prometheus metrics, and Pyroscope profiling
3. **Graceful Shutdown**: Handles SIGTERM with configurable grace period and lameduck mode
4. **Clean Architecture**: Types separated into `internal/types` package
5. **Production-Ready**: Connection limiting, HTTP keep-alive control, and minimal Docker image

### Configuration
Required flags:
- `--client-id`: GitHub App client ID
- `--private-key`: GitHub App private key (typically loaded from Kubernetes secret)

Optional flags:
- `--address`: Listen address (default: `0.0.0.0:8080`)
- `--termination-grace-period-seconds`: Graceful shutdown timeout (default: 10s)
- `--max-connections`: Connection limit (default: 65532)

The service reads environment variables from `.env` in debug mode for OpenTelemetry and logging configuration.