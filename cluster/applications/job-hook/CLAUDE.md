# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The job-hook is a Kubernetes mutating admission webhook that automatically injects creation timestamps into Job resources. It adds the annotation `job-hook.kaidotio.github.io/job-creation-timestamp` to Job pod templates, allowing Jobs to access their creation timestamp via environment variables using the Kubernetes downward API.

## Common Development Commands

### Development
- `make dev` - Primary development command that runs Skaffold with hot-reload and port forwarding

### Building
- `docker build .` - Build the Docker image locally

### Testing and Linting
Since this project doesn't have dedicated test files yet, use these standard Go commands:
- `go fmt ./...` - Format code according to Go standards
- `go vet ./...` - Run Go's static analyzer for suspicious constructs
- `go test ./...` - Run tests (when test files are added)
- `go mod tidy` - Clean up and verify module dependencies

For comprehensive linting (if needed):
- Install golangci-lint: `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2`
- Run: `golangci-lint run`

## High-Level Architecture

### Core Components
1. **Webhook Handler** (main.go): Processes admission requests for batch/v1 Job resources
   - Validates incoming requests
   - Adds timestamp annotation to Job pod templates
   - Returns admission response with patches

2. **Server Configuration**: 
   - Webhook server on port 9443 (TLS)
   - Health/readiness probes on port 8081
   - Metrics exposed on port 8080

3. **Deployment Model**:
   - Runs as a Deployment with 2 replicas
   - Uses cert-manager for TLS certificate management
   - Configured via MutatingWebhookConfiguration resource

### Key Design Patterns
1. **Controller-runtime based**: Built using the Kubernetes controller-runtime framework
2. **Fail-safe design**: Webhook failure policy set to "Ignore" to prevent blocking Job creation
3. **Security-hardened**: Runs as non-root with read-only filesystem and dropped capabilities
4. **Resource-aware**: Uses GOMAXPROCS and GOMEMLIMIT for proper resource management

### Development Workflow
1. Use `make dev` to deploy to local Kubernetes cluster with Skaffold
2. Webhook will automatically reload on code changes
3. Test by creating Jobs and checking for the injected annotation
4. Example Job with annotation usage is provided in `examples/annotation.yaml`