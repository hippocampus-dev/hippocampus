# Native Daemon Connection Caps

How to cap connections for a native proxy daemon (HAProxy, nginx) whose memory floor is set at startup.

## Why the Cap Belongs in the Daemon's Own Config

A proxy whose config omits a connection limit may derive one from the container's `RLIMIT_NOFILE` rather than from the cgroup — HAProxy does, nginx falls back to a fixed default.
Read the daemon's documented default rather than assuming.
Where it derives from the rlimit, the tables are allocated at startup, so RSS holds at the same figure on every replica regardless of load and reads as an inherent floor; `resources.limits.memory` cannot shrink it and only an explicit cap in the daemon's own config can.

## Placement

State the cap in every proxy config under `files/`.
Place it at the head of the file when an init script appends generated sections to the copied config (`init-haproxy.sh` does).

## Example

Copy from: `cluster/manifests/utilities/redis/files/haproxy.cfg` (`global maxconn`), `cluster/manifests/mimir/base/files/nginx.conf` (`worker_rlimit_nofile`, `worker_connections`)
