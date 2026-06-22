---
paths:
  - "files/etc/systemd/**"
---

* `Requires=` and `After=` are always used as a pair for dependencies
* `After=network-online.target` handles both startup (waits for network) and shutdown (stops before network) ordering
* Do NOT combine `After=network-online.target` with `Before=network.target` (causes ordering cycle)

## Service Placement

| Service Type | File Path | setup.sh Array | WantedBy |
|--------------|-----------|----------------|----------|
| System service | `files/etc/systemd/system/{name}.service` | `_SYSTEM_TARGETS` in `setup.sh` | `multi-user.target` |
| User service | `files/etc/systemd/user/{name}.service` | `_SYSTEM_TARGETS` in `setup.sh` (symlink) + `_USER_SERVICES` in `setup/arch/env.sh` (enable) | `default.target` |

User services require two entries: the symlink via `_SYSTEM_TARGETS` in `setup.sh` and the enablement via `_USER_SERVICES` in `setup/arch/env.sh`.

## Dependency Patterns

| Dependency Type | Requires | After |
|-----------------|----------|-------|
| Network (with or without ExecStop) | network-online.target | network-online.target |
| Other service | {service}.service | {service}.service |
| Docker | docker.service | docker.service |
| Libvirt (minikube, VMs) | libvirtd.service polkit.service | libvirtd.service polkit.service |

## Network Dependency

systemd stops services in reverse dependency order during shutdown. `After=network-online.target` ensures:
- Startup: service starts after network is online
- Shutdown: service stops before network-online.target is deactivated

```ini
[Unit]
Requires=network-online.target
After=network-online.target
```
