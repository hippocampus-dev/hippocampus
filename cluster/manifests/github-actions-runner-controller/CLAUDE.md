# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GitHub Actions Runner Controller is a Kubernetes operator that manages self-hosted GitHub Actions runners as Kubernetes pods. It provides automated lifecycle management of runners, dynamic image building with Kaniko, and supports both GitHub personal access tokens and GitHub Apps authentication.

This project has two main directories:
- **Application code**: `/opt/hippocampus/cluster/applications/github-actions-runner-controller/` - Contains Go source code, CRDs, and controller logic
- **Deployment manifests**: `/opt/hippocampus/cluster/manifests/github-actions-runner-controller/` - Contains Kustomize-based deployment configurations

## Common Development Commands

### Build and Code Generation
- `make all` - Generate CRDs from Go types using controller-gen (installs controller-gen v0.19.0)
- `go build -o bin/manager main.go` - Build the controller binary
- `go mod tidy` - Update Go dependencies
- `docker build -t github-actions-runner-controller .` - Build Docker image

### Local Development
- `make dev` - Creates a Kind cluster and runs Skaffold for hot-reload development
  - Creates cluster named `github-actions-runner-controller` using `kind.yaml` config
  - Runs `skaffold dev --port-forward` with automatic cleanup on exit
  - Mounts local Docker config into Kind nodes for registry access
- `skaffold dev --port-forward` - Run in existing cluster with port forwarding
- `kind create cluster --name github-actions-runner-controller --config kind.yaml` - Create local Kubernetes cluster
- `kind delete cluster --name github-actions-runner-controller` - Clean up local cluster

### Testing
- `go test ./...` - Run all tests (currently no test files exist)
- **Note**: When implementing tests, focus on:
  - Controller reconciliation logic in `internal/controllers/`
  - Token generation and expiration handling
  - Resource creation and cleanup
  - Error handling and retry logic

### Debugging and Troubleshooting
- `kubectl logs -n github-actions-runner-controller -l app.kubernetes.io/name=github-actions-runner-controller` - View controller logs
- `kubectl describe runner <name>` - Check Runner resource status
- `kubectl get pods -l app=<runner-name>` - List pods created by a Runner
- `kubectl logs <runner-pod-name> -c builder` - View Kaniko builder logs
- `kubectl logs <runner-pod-name> -c runner` - View GitHub Actions runner logs
- `kubectl port-forward -n github-actions-runner-controller svc/github-actions-runner-controller-metrics 8080:8080` - Access metrics

### Deployment with Kustomize
From the manifests directory (`/opt/hippocampus/cluster/manifests/github-actions-runner-controller/`):
- `kubectl apply -k base/` - Deploy base configuration
- `kubectl apply -k overlays/dev/` - Deploy with development overlay
- `kustomize build overlays/dev/` - Preview generated manifests

## High-Level Architecture

### Core Components

1. **Runner CRD** (`api/v1/runner_types.go`):
   - Defines the Runner custom resource with specs for GitHub repo, authentication, and runner configuration
   - Key fields:
     - `image`: Base image for the runner (e.g., `ubuntu:24.04`)
     - `owner`: GitHub organization or user
     - `repo`: GitHub repository name
     - `tokenSecretKeyRef`: Reference to GitHub PAT secret
     - `appSecretRef`: Reference to GitHub App credentials
   - Supports customization via `builderContainerSpec` and `runnerContainerSpec`
   - Allows pod template customization via `template` field

2. **Controller** (`internal/controllers/runner_controller.go`):
   - Reconciles Runner resources to create/update Kubernetes Deployments
   - Key responsibilities:
     - Creates/updates workspace ConfigMap with Dockerfile for runner image
     - Manages GitHub authentication tokens (PAT or GitHub App)
     - Creates Deployment with init container (Kaniko builder) and runner container
     - Handles token expiration and renewal logic (requeues 1 minute before expiry)
   - Uses SHA256 hash of image+versions for repository naming

3. **Manager** (`main.go`):
   - Entry point that sets up the controller-runtime manager
   - Configuration via flags/environment variables:
     - `--metrics-bind-address` (default: `0.0.0.0:8080`)
     - `--health-probe-bind-address` (default: `0.0.0.0:8081`)
     - `--enable-leader-election` (default: false)
     - `--push-registry-url` / `--pull-registry-url` (default: `ghcr.io/hippocampus-dev/hippocampus/github-actions-runner-controller`)
     - `--github-app-client-id`, `--github-app-installation-id`, `--github-app-private-key`
     - `--kaniko-image` (default: `gcr.io/kaniko-project/executor:v1.23.0`)
     - `--binary-version` (default: `0.1.0`)
     - `--runner-version` (default: `2.323.0`)
     - `--disableupdate` (default: false)
   - Implements HTTP/2 security controls

4. **Registry** (`manifests/stateful_set.yaml`):
   - Local Docker registry deployed as StatefulSet
   - Provides storage for Kaniko-built runner images
   - Uses persistent volume for image storage
   - Exposed via NodePort service for cluster-wide access

### Deployment Architecture

- Controller deployment configuration:
  - Single-replica deployment (controlled by manager's leader election)
  - Metrics exposed on port 8080, health probes on port 8081
  - Uses distroless image for security (`gcr.io/distroless/static:nonroot`)
  - Runs as non-root user (UID 65532)
  
- Each Runner resource creates:
  - **ConfigMap**: Contains Dockerfile for building runner image
  - **Secret** (if using GitHub App): Generated access token with expiration tracking
  - **Deployment**:
    - Init container: Kaniko builds custom runner image and pushes to registry
    - Main container: Runs GitHub Actions runner with generated configuration
    - Anti-affinity rules to spread runners across nodes
    - Security context with restricted capabilities

### Kustomize Structure

The manifests directory follows standard Kustomize patterns:

- **base/**: Core resources and patches
  - Controller deployment
  - Registry StatefulSet
  - RBAC resources
  - Image transformations
  
- **overlays/dev/**: Development environment customizations
  - Namespace configuration
  - GitHub App secret injection from Vault
  - Istio sidecar configuration
  - Network policies
  - Service entries for external access
  - Telemetry configuration

### Build Process

1. **Controller Build**:
   - Multi-stage Dockerfile using Go 1.24
   - BuildKit caching for dependencies and build artifacts
   - Produces minimal distroless image

2. **Runner Image Build** (at runtime):
   - Kaniko builds custom image based on user-specified base image
   - Installs runner binary from GitHub releases
   - Configures runner user (UID 60000) with sudo access
   - Pre-installs GitHub Actions runner software

### Authentication Methods

1. **Personal Access Token**: 
   - User creates secret with GitHub PAT
   - Specify `tokenSecretKeyRef` in Runner spec
   - Token mounted as file in runner container

2. **GitHub App**: 
   - Controller configured with app credentials
   - Automatically generates installation access tokens
   - Monitors token expiration and renews before expiry
   - Required permissions: actions (read), administration (read/write), metadata (read)

### Controller Patterns

- **Reconciliation Loop**:
  - Indexes ConfigMaps and Deployments by owner for efficient lookup
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
- `METRICS_BIND_ADDRESS`
- `METRICS_SECURE`
- `ENABLE_HTTP2`
- `HEALTH_PROBE_BIND_ADDRESS`
- `ENABLE_LEADER_ELECTION`
- `PUSH_REGISTRY_URL`
- `PULL_REGISTRY_URL`
- `GITHUB_APP_CLIENT_ID`
- `GITHUB_APP_INSTALLATION_ID`
- `GITHUB_APP_PRIVATE_KEY`
- `KANIKO_IMAGE`
- `BINARY_VERSION`
- `RUNNER_VERSION`
- `DISABLEUPDATE`

### Key Files and Their Roles

**Application Directory** (`/opt/hippocampus/cluster/applications/github-actions-runner-controller/`):
- `api/v1/`: CRD API definitions
  - `runner_types.go`: Runner CRD struct definitions
  - `groupversion_info.go`: API group registration
  - `zz_generated.deepcopy.go`: Generated DeepCopy methods
- `internal/controllers/runner_controller.go`: Core reconciliation logic
- `main.go`: Controller manager setup and configuration
- `Makefile`: Build automation (CRD generation)
- `Dockerfile`: Multi-stage build for controller
- `skaffold.yaml`: Local development configuration
- `kind.yaml`: Local Kubernetes cluster configuration
- `manifests/crd/`: Generated CRD YAML files
- `examples/`: Sample Runner resource configurations

**Manifests Directory** (`/opt/hippocampus/cluster/manifests/github-actions-runner-controller/`):
- `base/kustomization.yaml`: Base resource definitions
- `base/patches/`: Patches for deployment and StatefulSet
- `overlays/dev/`: Development environment overlay
  - `namespace.yaml`: Namespace definition
  - `secrets_from_vault.yaml`: Vault secret integration
  - `network_policy.yaml`: Network access controls
  - `sidecar.yaml`: Istio sidecar configuration
  - `service_entry.yaml`: External service access

### Common Deployment Scenarios

1. **Deploy with Personal Access Token**:
```bash
kubectl create secret generic github-token --from-literal=token=<YOUR_TOKEN>
kubectl apply -f - <<EOF
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: my-runner
spec:
  image: ubuntu:24.04
  owner: myorg
  repo: myrepo
  tokenSecretKeyRef:
    name: github-token
    key: token
EOF
```

2. **Deploy with GitHub App**:
```bash
# Controller must be configured with GitHub App credentials
kubectl apply -f - <<EOF
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: app-runner
spec:
  image: ubuntu:24.04
  owner: myorg
  repo: myrepo
  appSecretRef:
    name: github-app-generated  # Auto-generated by controller
EOF
```

3. **Deploy with Custom Resources**:
```bash
kubectl apply -f - <<EOF
apiVersion: github-actions-runner.kaidotio.github.io/v1
kind: Runner
metadata:
  name: resource-runner
spec:
  image: ubuntu:24.04
  owner: myorg
  repo: myrepo
  tokenSecretKeyRef:
    name: github-token
    key: token
  runnerContainerSpec:
    resources:
      requests:
        cpu: 2
        memory: 4Gi
      limits:
        cpu: 4
        memory: 8Gi
EOF
```