---
paths:
  - "**/docker-compose*.yaml"
  - "**/docker-compose*.yml"
---

* Use `include` in root `docker-compose.yaml` to organize profile-specific files
* Profile files go in `docker-compose/docker-compose.{service}.yaml`
* Always specify `profiles` for services in profile files
* Services without `profiles` go directly in root `docker-compose.yaml` (do not create a dedicated included file)
* Use explicit volume type syntax: `type: bind` or `type: volume`
* For bind mounts: set `bind.create_host_path: false` and `read_only: true` where appropriate
* Define all named volumes in root file's `volumes:` section
* Use `${VAR}` format for environment variables
* Encrypt sensitive values in `.enc` files
* Never give a service `ports:` while attaching it only to a network with `internal: true` - Docker publishes no port for such a container, so `docker compose up` succeeds and the compose file still reads as if the port were published while nothing on the host can connect (`armyknife-tmux` and `armyknife-notify` carry their own bridge for this reason)
* Pin a network's `ipam` subnet outside every pool Docker falls back to while `default-address-pools` is unset in `files/etc/docker/daemon.json`, reading those pools off the networks it assigned itself rather than assuming a range - the networks declared beside it draw from the same pools as the stack comes up, so a subnet pinned inside one is handed to whichever network is created first and the whole `docker compose up` fails with `Pool overlaps with other one on this address space` while nothing in the compose file names what it collided with (`internal` in the root `docker-compose.yaml` sits at `10.200.0.0/16` for this reason)
* Pin the version of a tool a service installs at startup rather than taking `@latest` - a language server tracks the newest major toolchain release, so `@latest` breaks the day upstream ships one the service's pinned image predates, and nothing builds or lints a `command:` block, leaving a container that exits at start-up as the only signal; bump the pin together with the image tag that `.claude/rules/dockerfile.md`'s `## Language Version Consistency` aligns to the dependency file
* Raise a downloader's image tag to the release that added support before adding a model to the `pull` list it runs - the list lives in a `command:` block nothing builds or lints, so a model the pinned runtime predates leaves the downloader exiting successfully with the failure only in its log, and the gap first shows up as an error from whatever consumes the model
* Give a service running a `ghcr.io/hippocampus-dev/hippocampus/*:main` image with no `build:` section `pull_policy: always` - `files/etc/systemd/system/hippocampus-compose.service` starts the stack with `docker compose up --build`, which reaches only the services that build, and `pull_policy` defaults to `missing` for the rest, so such a service keeps the copy it first pulled and runs code older than this repository's with nothing failing anywhere

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

## Adding HTTP Service

When adding a new HTTP service to docker-compose.yaml:

1. Add service to `docker-compose.yaml` with appropriate network
2. Add routing to `docker-compose/envoy/envoy.yaml`:
   - Add virtual_host entry with domain `{service}.127.0.0.1.nip.io`
   - Add cluster entry with service address and port
3. Do not expose `ports` for HTTP services -- Envoy handles external access, except a broker bridging the claudex sandbox to a host resource (last table row), which publishes a loopback-only port and is registered as an SSE MCP server in `setup/user.sh`; `.claude/reference/files/tmux.md` and the header comment in `files/home/kai/bin/claudex` own why the sandbox reaches it over the port rather than the host socket
4. If reusable in cluster: place Dockerfile in `cluster/applications/{name}/`

| Scenario | Network | `ports` |
|----------|---------|---------|
| HTTP-only access via Envoy | `internal` only | None |
| Also needs non-HTTP protocols (e.g., Redis wire protocol) | `default` and `internal` | Non-HTTP ports only |
| Broker bridging the claudex sandbox to a host resource (`armyknife-tmux`, `armyknife-notify`) | own dedicated bridge, no other members | `127.0.0.1:PORT` only |
