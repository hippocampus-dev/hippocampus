---
paths:
  - "files/etc/systemd/**"
---

* `Requires=` and `After=` are always used as a pair for dependencies
* `After=network-online.target` handles both startup (waits for network) and shutdown (stops before network) ordering
* Do NOT combine `After=network-online.target` with `Before=network.target` (causes ordering cycle)
* A `[Timer]` carries `OnCalendar=` and `Persistent=true`, so a run the host was powered off through fires once at the next boot rather than being skipped

## Unit Placement

| Unit | File Path | `_SYSTEM_TARGETS` entry | Enablement entry | `[Install]` |
|------|-----------|-------------------------|------------------|-------------|
| System service started at boot | `files/etc/systemd/system/{name}.service` | the `.service` | `_SERVICES` in `setup/arch/env.sh` | `WantedBy=multi-user.target` |
| User service started at login | `files/etc/systemd/user/{name}.service` | the `.service` | `_USER_SERVICES` in `setup/arch/env.sh` | `WantedBy=default.target` |
| Scheduled unit | `{name}.timer` beside the `{name}.service` it starts, in that same directory | both, the `.timer` listed first | the `.timer`, in whichever array that directory takes | `WantedBy=timers.target` on the `.timer`; the `.service` carries no `[Install]` |

Every unit needs both entries: `_SYSTEM_TARGETS` only places the symlink, and `setup.sh` enables `_SERVICES` and `_USER_SERVICES` alone, so a unit missing from those is installed but never started.
Only the `_SERVICES` loop passes `--now`, so a system unit has to start cleanly under the `[Service]` settings it is committed with - that loop runs ahead of `setup/user.sh` and the closing `reboot` under `set -Eeo pipefail`, so a unit failing to start ends provisioning there - while a `_USER_SERVICES` entry is enabled only and first starts at the next login.

## Dependency Patterns

| Dependency Type | Requires | After |
|-----------------|----------|-------|
| Network (with or without ExecStop) | network-online.target | network-online.target |
| Other service | {service}.service | {service}.service |
| Docker | docker.service | docker.service |
| Libvirt (minikube, VMs) | libvirtd.service polkit.service | libvirtd.service polkit.service |
