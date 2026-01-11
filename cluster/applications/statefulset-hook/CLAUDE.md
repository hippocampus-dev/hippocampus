# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

statefulset-hook is a Kubernetes mutating admission webhook that handles StatefulSet CREATE operations when orphaned pods exist. When a StatefulSet is created, the webhook checks if there are already running pods matching the StatefulSet's selector. If orphaned pods are found, it sets the StatefulSet's replicas to match the existing pod count, preventing the controller from creating duplicate pods.

## Common Development Commands

### Development
- `make dev` - Primary development command that runs Skaffold with hot-reload and port forwarding

### Building
- `docker build .` - Build the Docker image locally

### Testing and Linting
- `go fmt ./...` - Format code according to Go standards
- `go vet ./...` - Run Go's static analyzer
- `go test ./...` - Run tests (when test files are added)
- `go mod tidy` - Clean up and verify module dependencies
- `make all` - Run full test suite: format, lint, tidy, test

## High-Level Architecture

### Core Components
1. **Webhook Handler** (main.go): Processes admission requests for apps/v1 StatefulSet CREATE operations
   - Validates incoming requests are for StatefulSet CREATE
   - Queries for existing pods matching the StatefulSet's selector
   - Patches replicas to match existing pod count if orphaned pods exist

2. **Server Configuration**:
   - Webhook server on port 9443 (TLS)
   - Health/readiness probes on port 8081
   - Metrics exposed on port 8080

3. **Deployment Model**:
   - Runs as a Deployment with 2 replicas
   - Uses cert-manager for TLS certificate management
   - Configured via MutatingWebhookConfiguration resource

### Request Flow
1. Kubernetes API server sends admission review for StatefulSet CREATE
2. Webhook extracts the label selector from the StatefulSet
3. Lists pods in the same namespace matching the selector
4. Filters out terminating pods (DeletionTimestamp != nil) and succeeded pods
5. If matching pods exist, patches replicas to equal the pod count
6. Returns admission response (allowed with patches or allowed without changes)

### Key Design Patterns
1. **Controller-runtime based**: Built using the Kubernetes controller-runtime framework
2. **Fail-safe design**: Webhook failure policy set to "Fail" to ensure orphaned pods are handled
3. **Security-hardened**: Runs as non-root with read-only filesystem and dropped capabilities
4. **Orphan-aware**: Specifically designed to handle cases where pods exist before their StatefulSet

### Use Case
This webhook is useful when:
- StatefulSets are deleted with `--cascade=orphan` leaving pods running
- A new StatefulSet is created to adopt those orphaned pods
- Without this webhook, the new StatefulSet would try to create additional pods instead of adopting existing ones

### Development Workflow
1. Use `make dev` to deploy to local Kubernetes cluster with Skaffold
2. Webhook will automatically reload on code changes
3. Test by creating orphaned pods and then creating a StatefulSet with matching selector
4. Example StatefulSet is provided in `skaffold/examples/sample-statefulset.yaml`
