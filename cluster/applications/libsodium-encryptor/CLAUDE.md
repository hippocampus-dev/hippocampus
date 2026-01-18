# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

libsodium-encryptor is a Node.js microservice that provides HTTP-based encryption functionality using the libsodium cryptographic library (via tweetsodium). It exposes a simple REST API for encrypting values with public keys. This service is part of the larger Hippocampus Kubernetes platform.

## Common Development Commands

### Installation and Dependencies
```bash
npm ci                    # Install dependencies from package-lock.json
npm install              # Install/update dependencies
npm update              # Update dependencies to latest versions allowed by package.json
```

### Running the Service
```bash
node main.mjs           # Run the service locally (port 8080)
npm start               # Alternative if start script is added to package.json
```

### Building and Testing
```bash
# Build Docker image
docker build -t libsodium-encryptor .

# Run container locally
docker run -p 8080:8080 libsodium-encryptor

# Test the encryption endpoint
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"key":"<base64-public-key>","value":"test"}'

# Health check
curl http://localhost:8080/healthz
```

### Kubernetes Deployment
```bash
# Deploy using manifests (from cluster/manifests/libsodium-encryptor)
kubectl apply -k overlays/dev/

# Check deployment status
kubectl get pods -n libsodium-encryptor
kubectl logs -n libsodium-encryptor -l app=libsodium-encryptor
```

## Architecture

This is a minimal Express.js application with the following structure:

- **main.mjs**: Entry point containing the entire application logic
  - POST `/`: Encrypts a value with a provided public key
    - Request body: `{ "key": "<base64-encoded-public-key>", "value": "<string-to-encrypt>" }`
    - Response: Base64-encoded encrypted value
    - Returns 400 Bad Request for missing parameters or invalid key size
    - Returns 500 Internal Server Error for other encryption failures
  - GET `/healthz`: Health check endpoint returning "OK"
  
### Technology Stack
- `express` v5 - HTTP server framework
- `tweetsodium` - libsodium bindings for Node.js (uses `seal` function for public key encryption)
- Node.js 22 - Runtime environment
- Distroless base image - Minimal container footprint with non-root user (65532)

### Kubernetes Integration
- Deployed as a Deployment with HorizontalPodAutoscaler
- Service exposed internally for other services
- Includes Istio configuration for service mesh integration
- Network policies for secure communication
- Telemetry and monitoring via Istio

## Key Implementation Details

- **Encryption Method**: Uses libsodium's sealed box (anonymous sender public key encryption)
- **Input Validation**: Checks for required `key` and `value` parameters
- **Key Format**: Public keys must be base64-encoded in requests
- **Output Format**: Encrypted values returned as base64-encoded strings
- **Error Handling**: 
  - Specific handling for "bad public key size" errors (400)
  - Generic error logging and 500 response for other failures
- **Graceful Shutdown**: Handles SIGTERM signal for Kubernetes pod termination
- **Security**: Runs as non-root user in production, uses distroless image

## Development Notes

- No test suite currently exists - consider adding tests using Jest or Mocha
- No linting configuration - consider adding ESLint
- Package.json has no scripts section - consider adding standard npm scripts
- Consider adding request validation middleware for better error messages
- Could benefit from OpenAPI/Swagger documentation for the API