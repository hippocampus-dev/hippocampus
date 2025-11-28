# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OWASP ZAP Controller is a Kubernetes controller that enables declarative security scanning using OWASP ZAP. It manages ZAP scans through Custom Resource Definitions (CRDs) and integrates with ZAP's Automation Framework to perform various security tests including spider scans, active scans, and passive scans.

## Common Development Commands

### Build and Test
- `make build` - Build the controller binary
- `make test` - Run unit tests
- `make manifests` - Generate CRDs, RBAC manifests, and webhook configurations
- `make generate` - Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject methods
- `make fmt` - Format code
- `make vet` - Run go vet against code
- `make lint` - Run golangci-lint (requires golangci-lint installed)

### Local Development
- `make dev` - Deploy to Kind cluster with Skaffold (hot-reload enabled)
- `make install` - Install CRDs into the cluster
- `make run` - Run controller locally against the cluster
- `make uninstall` - Uninstall CRDs from the cluster
- `make deploy` - Deploy controller to the cluster
- `make undeploy` - Undeploy controller from the cluster

### Docker and Release
- `make docker-build` - Build docker image
- `make docker-push` - Push docker image
- `make docker-buildx` - Build and push multi-platform docker image
- `make build-installer` - Generate installer YAML with CRDs and controller

## High-Level Architecture

### Core Components

1. **Custom Resource Definition (CRD)**
   - `Scan` resource (`api/v1/scan_types.go`) defines security scan configurations
   - Supports multiple authentication types: Basic, Form, JWT, JSON, Script
   - Configurable scan types: Spider, Ajax Spider, Active Scan, Passive Scan
   - Output formats: JSON, HTML, XML, Markdown, SARIF

2. **Controller Pattern**
   - Main reconciliation logic in `internal/controller/scan_controller.go`
   - Uses generation tracking to detect spec changes
   - Creates Kubernetes Jobs to run ZAP scans
   - Stores scan results in ConfigMaps
   - Maintains scan history (last 10 scans)

3. **ZAP Integration**
   - Uses ZAP Automation Framework via YAML configuration
   - Runs ZAP in official Docker container (`ghcr.io/zaproxy/zaproxy:stable`)
   - Configuration generation in `internal/controller/zap_config.go`
   - Supports advanced features: contexts, users, scripts, technologies

### Key Design Patterns

1. **Resource Management**
   - Scan CRD owns Jobs and ConfigMaps via owner references
   - Automatic cleanup of old scan results (keeps latest 10)
   - Generation-based change detection prevents unnecessary scans

2. **Configuration Generation**
   - Dynamic ZAP automation YAML generation based on CRD spec
   - Flexible authentication configuration support
   - Context and session management for authenticated scans

3. **Result Handling**
   - Multiple output format support with type-specific processing
   - Results stored in labeled ConfigMaps for easy retrieval
   - Status updates with scan progress and completion information

## Testing Patterns

- Unit tests use Ginkgo/Gomega framework
- Controller tests use envtest for Kubernetes API simulation
- Test files follow `*_test.go` naming convention
- Example scan configurations in `config/samples/`

## Important Implementation Details

1. **Authentication Handling**
   - Each auth type has specific configuration requirements
   - Script-based auth allows custom authentication flows
   - Session management for maintaining authenticated state

2. **Scan Lifecycle**
   - Scans triggered by spec changes (generation increment)
   - Job creation with appropriate ZAP configuration
   - Result retrieval and storage upon job completion
   - Status updates throughout the process

3. **Error Handling**
   - Validation of scan configurations
   - Job failure detection and reporting
   - Graceful handling of missing or invalid configurations

## Security Considerations

- Secrets referenced in scan specs must exist in the same namespace
- Authentication credentials handled securely via Kubernetes secrets
- No credential logging or exposure in scan outputs
- Container runs with minimal privileges