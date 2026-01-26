# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PersistentVolumeClaim Hook is a Kubernetes mutating admission webhook that prevents storage size reduction for PersistentVolumeClaims and VolumeClaimTemplates in StatefulSets. It intercepts UPDATE operations and ensures storage requests cannot be decreased from their previous values to prevent data loss.

## Common Development Commands

- `make dev` - Primary development command that runs `skaffold dev --port-forward` for local development with hot-reload
- `go mod tidy` - Tidy Go module dependencies
- `go test ./...` - Run all tests

## High-Level Architecture

### Core Components

1. **Mutating Admission Webhook** - The main handler that intercepts Kubernetes API requests
2. **Controller-Runtime Framework** - Provides the webhook server infrastructure, metrics, and health probes
3. **Certificate Management** - Uses cert-manager for TLS certificate provisioning

### Request Flow

1. Kubernetes API server sends admission review requests to the webhook for PVC/StatefulSet updates
2. The webhook handler decodes both the old and new resource objects
3. For PVCs: Compares `spec.resources.requests.storage` values
4. For StatefulSets: Compares storage in `spec.volumeClaimTemplates`
5. If new storage < old storage, creates a JSON patch to restore the original value
6. Returns an admission response (allowed with patches or allowed without changes)

### Key Implementation Details

- The webhook runs on port 9443 (HTTPS) with metrics on 8080 and health probes on 8081
- Uses JSON Patch (RFC 6902) format for modifications
- Failure policy is set to "Fail" - blocks updates if webhook is unavailable
- Runs with 2 replicas for high availability
- Certificate directory: `/var/k8s-webhook-server/serving-certs`

### Kubernetes Resources Structure

- **manifests/** - Base Kubernetes resources
  - `deployment.yaml` - Main webhook deployment
  - `mutating_webhook_configuration.yaml` - Webhook registration
  - `certificate.yaml` & `issuer.yaml` - TLS certificate configuration
- **skaffold/** - Development environment overrides
  - Patches for local development (namespace, certificates, webhook config)

### Development Workflow

1. Ensure you have a local Kubernetes cluster with cert-manager installed
2. Run `make dev` to start Skaffold development mode
3. Skaffold will:
   - Build the Docker image on file changes
   - Deploy to the `persistentvolumeclaim-hook` namespace
   - Set up port-forwarding for local access
   - Stream logs from the webhook pods
4. Test with the provided `sample-pvc.yaml` and `sample-pod.yaml` files

The webhook uses controller-runtime's webhook builder pattern, which handles most of the boilerplate for admission webhook implementation. The core business logic is in the handler functions that validate storage size changes.