---
paths:
  - "files/etc/systemd/**"
---

* `Requires=` and `After=` are always used as a pair for dependencies, except that a timer-started unit takes `Requisite=` in place of `Requires=` for a dependency stopped on purpose, and its `.timer` takes `PartOf=` on that dependency - `Requires=` starts the stopped dependency again, and a start during its stop job cancels the stop (`files/etc/systemd/system/minikube-worker.service`)
* `After=network-online.target` handles both startup (waits for network) and shutdown (stops before network) ordering
* Do NOT combine `After=network-online.target` with `Before=network.target` (causes ordering cycle)
* A `[Timer]` carries `OnCalendar=` and `Persistent=true`, so a run the host was powered off through fires once at the next boot rather than being skipped
* A user unit mounting a FUSE filesystem mounts it outside `/home/{user}` (`files/etc/systemd/user/rclone-gdrive.service`), as `## Choosing where a directory is created` in `.claude/rules/setup.md` sets out

## Unit Placement

| Unit | File Path | `_SYSTEM_TARGETS` entry | Enablement entry | `[Install]` |
|------|-----------|-------------------------|------------------|-------------|
| System service started at boot | `files/etc/systemd/system/{name}.service` | the `.service` | `_SERVICES` in `setup/arch/env.sh` | `WantedBy=multi-user.target` |
| User service started at login | `files/etc/systemd/user/{name}.service` | the `.service` | `_USER_SERVICES` in `setup/arch/env.sh` | `WantedBy=default.target` |
| Scheduled unit | `{name}.timer` beside the `{name}.service` it starts, in that same directory | both, the `.timer` listed first | the `.timer`, in whichever array that directory takes | `WantedBy=timers.target` on the `.timer`; the `.service` carries no `[Install]` |
| Template started only by another unit's `OnFailure=` | `files/etc/systemd/{system,user}/{name}@.service` | the `@.service` | none | none |
| Drop-in | `files/etc/systemd/{directory}/{unit or type}.d/{name}.conf` | each `.conf`, never the `.d` directory | none | none |

Every unit whose row names an enablement entry needs both entries: `_SYSTEM_TARGETS` only places the symlink, and `setup.sh` enables `_SERVICES` and `_USER_SERVICES` alone, so a unit missing from those is installed but never started - switch one off by commenting its enablement entry out in place (`#backup.timer` in `setup/arch/env.sh`) rather than deleting the line, since this sentence reads a missing entry as an omission to fix and restoring it starts the unit again.
Only the `_SERVICES` loop passes `--now`, so a system unit has to start cleanly under the `[Service]` settings it is committed with - that loop runs ahead of `setup/user.sh` and the closing `reboot` under `set -Eeo pipefail`, so a unit failing to start ends provisioning there - while a `_USER_SERVICES` entry is enabled only and first starts at the next login.

## Dependency Patterns

| Dependency Type | Requires | After |
|-----------------|----------|-------|
| Network (with or without ExecStop) | network-online.target | network-online.target |
| Other service | {service}.service | {service}.service |
| Docker | docker.service | docker.service |
| Libvirt (minikube, VMs) | libvirtd.service polkit.service | libvirtd.service polkit.service |
| Minikube from a timer-started unit | minikube.service (as `Requisite=`) | minikube.service |
