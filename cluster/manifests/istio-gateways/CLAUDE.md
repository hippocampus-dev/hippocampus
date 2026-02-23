# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Istio Gateway configurations for the Hippocampus cluster. It manages ingress traffic control, security policies, and observability settings using Istio's service mesh capabilities.

## Common Development Commands

### Building and Applying Manifests
- `kubectl apply -k overlays/dev/` - Apply the dev environment configuration
- `kubectl apply -k base/` - Apply only the base configuration (not recommended for production)
- `kustomize build overlays/dev/` - Build and view the generated manifests without applying

### Testing and Validation
- `kubectl apply -k overlays/dev/ --dry-run=client` - Validate manifests without applying
- `kustomize build overlays/dev/ | kubectl diff -f -` - Show what changes would be applied

## High-Level Architecture

### Directory Structure
- **`base/`** - Contains the core Istio gateway configurations:
  - `gateway.yaml` - Defines the cluster-local-gateway for internal traffic
  - `authorization_policy.yaml` - OAuth2-Proxy authentication configuration
  - `envoy_filter.yaml` - Request/response header transformations and security settings
  - `telemetry.yaml` - Access logging configuration for HTTP/1.0
  - `kustomization.yaml` - Base resource declarations

- **`overlays/dev/`** - Environment-specific configurations:
  - Applies namespace: `istio-gateways`
  - Patches authorization policy to add specific host authentication rules

### Key Components

1. **Gateway Configuration**
   - Cluster-local gateway on port 80 for internal HTTP traffic
   - Accepts all hosts (`*`)

2. **Security Features**
   - OAuth2-Proxy integration for external authentication
   - Security headers (HSTS, X-Frame-Options, X-Content-Type-Options, X-XSS-Protection)
   - Client IP extraction from X-Forwarded-For header
   - Report-To and NEL headers for error reporting

3. **Observability**
   - Traceparent header propagation for distributed tracing
   - Access logging for HTTP/1.0 requests
   - Envoy header cleanup (removes internal debugging headers)

4. **Header Processing**
   - X-Forwarded-For trust configuration (1 hop)
   - Server header passthrough
   - Response header enrichment

### Integration Points
- Works with `istio-ingressgateway` for external traffic handling
- Integrates with OAuth2-Proxy service for authentication
- Compatible with distributed tracing systems via traceparent headers