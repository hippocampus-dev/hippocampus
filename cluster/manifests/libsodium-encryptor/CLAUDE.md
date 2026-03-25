# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

libsodium-encryptor is a Node.js microservice that provides HTTP-based encryption functionality using the libsodium cryptographic library (via tweetsodium). It exposes a REST API for encrypting values with public keys and is deployed as part of the Hippocampus Kubernetes platform.

## Common Development Commands

### Local Development
```bash
# Install dependencies
npm ci

# Run locally (listens on port 8080)
node main.mjs

# Test encryption endpoint
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"key":"<base64-public-key>","value":"test"}'

# Health check
curl http://localhost:8080/healthz
```

### Docker Build
```bash
# Build image
docker build -t libsodium-encryptor .

# Run container
docker run -p 8080:8080 libsodium-encryptor
```

### Kubernetes Deployment
```bash
# Deploy to dev environment (from cluster/manifests/libsodium-encryptor)
kubectl apply -k overlays/dev/

# Check deployment
kubectl get pods -n libsodium-encryptor
kubectl logs -n libsodium-encryptor -l app=libsodium-encryptor
```

## Architecture

### Service Structure
- **main.mjs**: Single-file Express application
  - POST `/`: Encrypts value with public key
    - Request: `{ "key": "<base64-public-key>", "value": "<string>" }`
    - Response: Base64-encoded encrypted value
    - Error codes: 400 (bad request/invalid key), 500 (server error)
  - GET `/healthz`: Health check endpoint

### Technical Stack
- Node.js 22 with ES modules
- Express v5
- tweetsodium (libsodium wrapper)
- Distroless container image
- Non-root user (UID 65532)

### Kubernetes Integration
- Namespace: `libsodium-encryptor`
- Kustomize-based deployment (base + dev overlay)
- HorizontalPodAutoscaler configuration
- PodDisruptionBudget for availability
- Istio service mesh integration:
  - Sidecar injection
  - mTLS via PeerAuthentication
  - Telemetry configuration
  - Virtual service routing
- Network policies for secure communication

## Key Implementation Details

- **Encryption**: Uses libsodium's sealed box (anonymous sender public key encryption)
- **Input Validation**: Validates presence of `key` and `value` parameters
- **Error Handling**: Specific handling for "bad public key size" errors
- **Graceful Shutdown**: Handles SIGTERM for Kubernetes pod termination
- **Security**: Runs as non-root user, uses minimal distroless base image