# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the alerthandler service, which is a Knative Serving webhook receiver that processes alerts from Prometheus Alertmanager. The actual application source code is located at `/opt/hippocampus/cluster/applications/alerthandler/`.

## Common Development Commands

### Manifest Management
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests
- `kustomize build overlays/dev/` - Build manifests without kubectl

### Debugging and Monitoring
- `kubectl logs -n alerthandler -l app.kubernetes.io/name=alerthandler` - View alerthandler logs
- `kubectl get ksvc -n alerthandler` - Check Knative Service status

## High-Level Architecture

### Kustomize Structure
- **`base/`** - References the base manifests from the applications directory
- **`overlays/dev/`** - Development environment customizations including:
  - Namespace configuration
  - Network policies for secure ingress
  - Prometheus metrics configuration

### Network Policy Configuration

The network policies implement a defense-in-depth strategy:

1. **default-deny** - Blocks all ingress traffic by default
2. **allow-alertmanager-webhook** - Permits Alertmanager (from mimir namespace) to send webhook requests on port 8080
3. **allow-knative-serving** - Permits Knative activator to route traffic for scale-from-zero
4. **allow-envoy-stats-scrape** - Permits Prometheus to scrape Envoy sidecar metrics

### Istio Configuration

1. **PeerAuthentication** - Enforces STRICT mTLS with PERMISSIVE mode on port 8080 for Alertmanager webhook
2. **Sidecar** - REGISTRY_ONLY egress policy, allowing access to:
   - Same namespace services
   - Istio control plane (istiod)
   - OpenTelemetry agent
   - Kubernetes API server
3. **Telemetry** - Enables tracing (100% sampling), Prometheus metrics, and Envoy access logging

### Security Considerations

- Only Alertmanager from the mimir namespace can trigger alert handlers
- Knative serving activator is allowed for proper autoscaling behavior
- All other ingress traffic is denied by default
- mTLS enforced for service-to-service communication
- Egress restricted to known services only
- The service runs with strict security context (non-root, read-only filesystem, no privilege escalation)
