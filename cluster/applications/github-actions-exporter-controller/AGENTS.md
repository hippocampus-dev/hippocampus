# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GitHub Actions Exporter Controller is a Kubernetes operator that manages Prometheus exporters for GitHub Actions metrics. Each Exporter custom resource creates a Deployment running the `github-actions-exporter` binary that collects workflow run metrics from the GitHub API. The controller supports both GitHub personal access tokens and GitHub Apps authentication, with centralized credential management and per-CR overrides.

## Common Development Commands

### Build and Code Generation
- `make all` - Generate CRDs from Go types using controller-gen (installs controller-gen v0.19.0)
- `go build -o bin/manager main.go` - Build the controller binary
- `go mod tidy` - Update Go dependencies
- `docker build -t github-actions-exporter-controller .` - Build Docker image

### Local Development
- `make dev` - Creates a Kind cluster and runs Skaffold for hot-reload development
  - Creates cluster named `github-actions-exporter-controller` using `kind.yaml` config
  - Runs `skaffold dev --port-forward` with automatic cleanup on exit
  - Mounts local Docker config into Kind nodes for registry access
- `skaffold dev --port-forward` - Run in existing cluster with port forwarding
- `kind create cluster --name github-actions-exporter-controller --config kind.yaml` - Create local Kubernetes cluster
- `kind delete cluster --name github-actions-exporter-controller` - Clean up local cluster

### Testing
- `go test ./...` - Run all tests
- `go test -v ./internal/controllers/` - Test controller logic

### Deployment
- Uses Kustomize for manifest management (see `skaffold/` directory)
- Skaffold builds with input digest tagging policy and BuildKit enabled
- Controller image: `ghcr.io/hippocampus-dev/hippocampus/skaffold`

## High-Level Architecture

### Core Components

1. **Exporter CRD** (`api/v1/exporter_types.go`):
   - Defines the Exporter custom resource with specs for GitHub repo/organization and authentication
   - Key fields:
     - `owner`: GitHub organization or user (required)
     - `repo`: GitHub repository name (optional - determines scope)
     - `tokenSecretKeyRef`: Reference to GitHub PAT secret
     - `appSecretRef`: Reference to GitHub App credentials
     - `template`: Pod template customization (metadata only)
   - Scope is inferred from the `repo` field:
     - If `repo` is set: repository-level metrics
     - If `repo` is not set: organization-level metrics
   - Container resource requests are hardcoded in the controller (not customizable): cpu 10m, memory 16Mi

2. **Controller** (`internal/controllers/exporter_controller.go`):
   - Reconciles Exporter resources to create/update Kubernetes Deployments and Services
   - Key responsibilities:
     - Creates/updates Secret with GitHub access token (when using GitHub App)
     - Creates Deployment running the `github-actions-exporter` image
     - Creates Service for Prometheus scraping
     - Handles token expiration and renewal logic (requeues 1 minute before expiry)
     - Injects pod template annotation (`github-actions-exporter.kaidotio.github.io/expiresAt`) with token expiration time to trigger rolling updates when tokens are refreshed
   - Tracks token expiration via annotation on both Secret and pod template

3. **Manager** (`main.go`):
   - Entry point that sets up the controller-runtime manager
   - Configuration via flags/environment variables:
     - `--metrics-bind-address` (default: `0.0.0.0:8080`)
     - `--health-probe-bind-address` (default: `0.0.0.0:8081`)
     - `--enable-leader-election` (default: false)
     - `--github-app-client-id`, `--github-app-installation-id`, `--github-app-private-key`
     - `--exporter-image` (default: `ghcr.io/hippocampus-dev/hippocampus/github-actions-exporter`)
   - Implements HTTP/2 security controls

### Deployment Architecture

- Controller deployment configuration:
  - Two replicas with leader election for high availability
  - Metrics on containerPort 8080, health probes on containerPort 8081 (no Service; scraped via Istio sidecar port 15020)
  - Uses distroless image for security (`gcr.io/distroless/static:nonroot`)
  - Runs as non-root user (UID 65532)

- Each Exporter resource creates:
  - **Secret** (if using GitHub App): Generated access token with expiration tracking
  - **Deployment**: Single-replica running the exporter binary
  - **Service**: ClusterIP service for Prometheus scraping (port 8080)

### Authentication Methods

1. **Personal Access Token**:
   - User creates secret with GitHub PAT
   - Specify `tokenSecretKeyRef` in Exporter spec
   - Token mounted via `envFrom` in exporter container

2. **GitHub App** (centralized):
   - Controller configured with app credentials via flags/environment
   - Automatically generates installation access tokens
   - Monitors token expiration and renews before expiry
   - Creates/updates Secret in exporter namespace with token

3. **GitHub App** (per-CR override):
   - Specify `appSecretRef` in Exporter spec to override controller-level auth
   - Controller generates token using credentials from referenced secret

### Controller Patterns

- **Reconciliation Loop**:
  - Indexes Secrets, Services, and Deployments by owner for efficient lookup
  - Implements optimistic locking retry for concurrent updates
  - Generation-based change detection to avoid unnecessary reconciliations

- **Resource Management**:
  - Owner references ensure automatic cleanup of child resources
  - Cleanup logic removes orphaned resources during reconciliation
  - Single concurrent reconcile to prevent race conditions

- **Error Handling**:
  - Returns errors for requeue with exponential backoff
  - Optimistic lock conflicts trigger immediate retry after 1 second
  - Token expiration triggers requeue before expiry time

### Environment Variables Used

The controller uses `envOrDefaultValue` helper to read configuration from environment:
- `VARIANT` - API group prefix for CRD (used in skaffold development to avoid conflicts with production CRDs)
- `METRICS_BIND_ADDRESS`
- `METRICS_SECURE`
- `ENABLE_HTTP2`
- `HEALTH_PROBE_BIND_ADDRESS`
- `ENABLE_LEADER_ELECTION`
- `GITHUB_APP_CLIENT_ID`
- `GITHUB_APP_INSTALLATION_ID`
- `GITHUB_APP_PRIVATE_KEY`
- `EXPORTER_IMAGE`

### Key Files and Their Roles

- `api/v1/`: CRD API definitions
  - `exporter_types.go`: Exporter CRD struct definitions
  - `groupversion_info.go`: API group registration
  - `zz_generated.deepcopy.go`: Generated DeepCopy methods
- `internal/controllers/exporter_controller.go`: Core reconciliation logic
- `main.go`: Controller manager setup and configuration
- `Makefile`: Build automation (CRD generation)
- `Dockerfile`: Multi-stage build for controller
- `skaffold.yaml`: Local development configuration
- `kind.yaml`: Local Kubernetes cluster configuration
- `manifests/crd/`: Generated CRD YAML files

### Relationship with github-actions-exporter

This controller manages instances of the `github-actions-exporter` binary (located in `cluster/applications/github-actions-exporter/`). The exporter binary:
- Collects GitHub Actions workflow run metrics via the GitHub API on each `/metrics` request
- Exposes Prometheus metrics on port 8080 at `/metrics`
- Supports environment variables: `GITHUB_OWNER`, `GITHUB_REPO`, `GITHUB_TOKEN`

The controller creates Deployments that run this exporter binary with appropriate environment configuration.
