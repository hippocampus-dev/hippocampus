---
paths:
  - "files/etc/systemd/system/**"
---

* Name an ancestor of a FUSE mount a user unit mounts in `InaccessiblePaths=` (`-/mnt` in `files/etc/systemd/system/docker-rootless.service`), never the mountpoint itself - PID 1 builds the namespace as root, which the live mount answers with `EACCES` as the FUSE mountpoint row of `## Choosing where a directory is created` in `.claude/rules/setup.md` records, and `apply_one_mount` in systemd's `src/core/namespace.c` skips a `-` entry only on `ENOENT`, so the unit fails with `226/NAMESPACE`
