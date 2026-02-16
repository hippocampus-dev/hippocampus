# ext-proc

Envoy External Processor gRPC service for header/body manipulation.

## When to Use

- Modifying request headers based on request body content
- Modifying response headers and body together
- Use cases where proxy-wasm cannot modify headers during body phases

## Example

| Type | Copy from |
|------|-----------|
| Request processing | `envoy-request-hasher` |
| Response processing | `envoy-markdownify` |

## Files (cluster/applications/{app}/)

| Directory | File | Purpose |
|-----------|------|---------|
| (root) | Dockerfile | Protobuf-based Rust gRPC service |
| (root) | e2e.sh | e2e test orchestration script |
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | deployment.yaml | gRPC server pod |
| manifests/ | service.yaml | ClusterIP service for gRPC |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/patches/ | deployment.yaml | Development overrides |
| e2e/ | skaffold.yaml | e2e Skaffold config (build context: `..`) |
| e2e/ | kustomization.yaml | e2e namespace and configMapGenerator |
| e2e/ | namespace.yaml | e2e namespace |
| e2e/ | deployment.yaml | Three-container pod (Envoy + ext-proc + httpbin) |
| e2e/ | service.yaml | ClusterIP for port-forward |
| e2e/files/ | envoy.yaml | Envoy config with ext_proc filter |
| k6/ | index.js | k6 load test script |

## Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/deployment.yaml`: Update container name, gRPC port, CLI args
- `e2e/deployment.yaml`: Update container names, images, CLI args
- `e2e/files/envoy.yaml`: Update `processing_mode` and cluster name
- `e2e/kustomization.yaml`: Update namespace and image name
- `e2e/skaffold.yaml`: Update image name
- `e2e.sh`: Update service name, namespace, k6 test paths
- `skaffold/kustomization.yaml`: Set namespace, add `- path: patches/deployment.yaml` to patches
- `skaffold/namespace.yaml`: Set namespace name
- `skaffold/patches/deployment.yaml`: Update deployment name and container name to match the application

## e2e Three-Container Pattern

The e2e deployment uses a single pod with three containers:

| Container | Image | Purpose |
|-----------|-------|---------|
| envoy | `envoyproxy/envoy` | Proxy with ext_proc filter configuration |
| {app-name} | App image | gRPC ext-proc service under test |
| httpbin | `kennethreitz/httpbin` | Backend to verify header modifications |

Envoy exposes two listeners:

| Port | ext_proc | Purpose |
|------|----------|---------|
| 8080 | Enabled | Test with ext-proc processing |
| 8081 | Disabled | Baseline comparison |

## e2e Skaffold Config

The `e2e/skaffold.yaml` uses `context: ..` to build from the parent directory:

```yaml
build:
  artifacts:
    - image: ghcr.io/hippocampus-dev/hippocampus/{app-name}
      context: ..
      docker:
        dockerfile: Dockerfile
manifests:
  kustomize:
    paths:
      - .
```

The `e2e/kustomization.yaml` uses `configMapGenerator` to inject the Envoy config:

```yaml
namespace: e2e-{app-name}
configMapGenerator:
- name: envoy
  files:
  - files/envoy.yaml
```
