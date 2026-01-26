# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the `snapshot-diff-server` microservice, which provides HTTP endpoints for calculating visual and textual differences between snapshots. The service is part of the larger snapshot-controller system.

## Architecture

### Service Purpose
The snapshot-diff-server is a stateless HTTP service that:
- Compares two images using pixel-level or rectangle-based diff algorithms
- Compares text content using line-based or DOM-based diff algorithms
- Returns base64-encoded diff images/data and a numerical diff amount
- Supports formats: `pixel`, `rectangle`, `line`, and `dom`

### Source Code Location
The actual Go application code lives at `/opt/hippocampus/cluster/applications/snapshot-controller/bin/diff-server/`

### Manifest Structure
```
base/                           # Base Kubernetes resources
  ├── deployment.yaml           # Main deployment with security hardening
  ├── service.yaml              # ClusterIP service on port 8080
  ├── horizontal_pod_autoscaler.yaml
  └── pod_disruption_budget.yaml

overlays/dev/                   # Development environment overlay
  ├── patches/                  # Kustomize patches for dev-specific config
  ├── gateway.yaml              # Istio Gateway configuration
  ├── virtual_service.yaml      # Istio routing rules
  ├── network_policy.yaml
  ├── peer_authentication.yaml
  ├── sidecar.yaml
  └── telemetry.yaml
```

## Key Configuration

### Container Image
- Registry: `ghcr.io/hippocampus-dev/hippocampus/snapshot-controller/snapshot-diff-server`
- Images are referenced by SHA256 digest in `base/kustomization.yaml`

### Service Endpoints
- **HTTP**: Port 8080
  - `POST /diff` - Main diff endpoint (accepts multipart form with baseline/target files and format parameter)
  - `GET /healthz` - Health check
  - `GET /metrics` - Prometheus metrics

### Security Hardening
The deployment follows security best practices:
- Runs as non-root user (UID 65532)
- Read-only root filesystem
- All capabilities dropped
- No privilege escalation
- RuntimeDefault seccomp profile
- No service account token auto-mount

### Observability Stack
Configured to integrate with:
- **OpenTelemetry**: Traces exported to `otel-agent.otel.svc.cluster.local:4317`
- **Pyroscope**: Continuous profiling to `pyroscope-distributor.pyroscope.svc.cluster.local:4040`
- **Prometheus**: Metrics exposed at `/metrics` with exemplar support
- **Istio**: Service mesh sidecar injection enabled

## Common Operations

### Updating Image Digests
When a new version is built, update the digest in `base/kustomization.yaml`:
```yaml
images:
- digest: sha256:NEW_DIGEST_HERE
  name: ghcr.io/hippocampus-dev/hippocampus/snapshot-controller/snapshot-diff-server
  newName: ghcr.io/hippocampus-dev/hippocampus/snapshot-controller/snapshot-diff-server
```

### Deploying Changes
After modifying manifests:
```bash
kubectl apply -k overlays/dev/
```

### Testing the Service
The service accepts POST requests with multipart form data:
- `baseline` (file): The baseline image/text
- `target` (file): The target image/text to compare
- `format` (string): One of `pixel`, `rectangle`, `line`, or `dom`

### Viewing Logs
```bash
kubectl logs -n snapshot-diff-server -l app.kubernetes.io/name=snapshot-diff-server -f
```

## Integration with Parent Project

This service is part of the snapshot-controller system:
- **Main controller**: `/opt/hippocampus/cluster/applications/snapshot-controller/` (Go-based Kubernetes operator)
- **Controller manifests**: `/opt/hippocampus/cluster/manifests/snapshot-controller/`
- The controller captures screenshots via Playwright and uses this diff-server to compare them
- Storage backend: S3-compatible storage (configured via `S3_BUCKET` environment variable in controller)

## Environment Variables (Application)

Key environment variables supported by the diff-server application:
- `ADDRESS`: Listen address (default: `0.0.0.0:8080`)
- `TERMINATION_GRACE_PERIOD`: Graceful shutdown timeout (default: 10s)
- `LAMEDUCK`: Pre-shutdown delay (default: 1s)
- `HTTP_KEEPALIVE`: Enable HTTP keep-alive (default: true)
- `MAX_CONNECTIONS`: Maximum concurrent connections (default: 65532)
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry collector endpoint
- `PYROSCOPE_ENDPOINT`: Pyroscope profiler endpoint
- `GO_LOG`: Log level (INFO, DEBUG, etc.)

## Istio Configuration

The service is accessed via Istio ingress:
- Hosts: `snapshot-diff-server.minikube.127.0.0.1.nip.io`, `snapshot-diff-server.kaidotio.dev`
- Automatic retries on transient failures
- Sidecar resource limits: 1000m CPU / 1Gi memory
