---
paths:
  - "**/docker-compose*.yaml"
  - "**/docker-compose*.yml"
---

* Use `include` in root `docker-compose.yaml` to organize profile-specific files
* Profile files go in `docker-compose/docker-compose.{service}.yaml`
* Always specify `profiles` for services in profile files
* Use explicit volume type syntax: `type: bind` or `type: volume`
* For bind mounts: set `bind.create_host_path: false` and `read_only: true` where appropriate
* Define all named volumes in root file's `volumes:` section
* Use `${VAR}` format for environment variables
* Encrypt sensitive values in `.enc` files

## Service Patterns

| Pattern | Use Case |
|---------|----------|
| Main service | Primary application container |
| Chown service | Fix volume permissions for non-root (UID 65532) |
| Downloader service | Pre-download models or dependencies |

## GPU Services

```yaml
services:
  app:
    runtime: nvidia
    # deploy block is optional, runtime is preferred
```

## Development Watch

```yaml
develop:
  watch:
    - action: rebuild
      path: {service}/Dockerfile
```

## Dependency Management

| Condition | When to Use |
|-----------|-------------|
| `service_started` | Service is running |
| `service_completed_successfully` | One-shot task finished |
| `service_healthy` | Health check passed |

## Volume Mount Format

```yaml
volumes:
  # Bind mount (read-only config)
  - type: bind
    bind:
      create_host_path: false
    source: ./path/to/file
    target: /container/path
    read_only: true
  # Named volume (persistent data)
  - type: volume
    source: volume-name
    target: /container/path
```

## Network Format

```yaml
networks:
  - default        # External access
  - internal       # Internal only
```

## Adding HTTP Service

When adding a new HTTP service to docker-compose.yaml:

1. Add service to `docker-compose.yaml` with `networks: default, internal`
2. Add routing to `docker-compose/envoy/envoy.yaml`:
   - Add virtual_host entry with domain `{service}.127.0.0.1.nip.io`
   - Add cluster entry with service address and port
3. If reusable in cluster: place Dockerfile in `cluster/applications/{name}/`
