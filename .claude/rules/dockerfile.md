---
paths:
  - "**/Dockerfile*"
---

* Always start with `# syntax=docker/dockerfile:1.4`
* Add `LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"` after runtime stage FROM
* Use multi-stage builds: `builder` stage + runtime stage
* Run as non-root user (UID 65532)
* Use `--mount=type=cache` for package manager caches
* Use `cp` instead of `mv` when copying from `--mount=type=cache` targets (preserves cache; `mv` destroys it)
* Use `install -m 755` to combine `mv` + `chmod +x` into a single command when both renaming and setting permissions
* Prefer individual `COPY` commands (`COPY src /opt/builder/src`) over `COPY . .` to avoid unintended files in the build context
* Include `.dockerignore` only when using `COPY .` — individual `COPY` commands do not need it

## Reference

If writing a Dockerfile for a specific language:
  Read: `.claude/rules/.reference/dockerfile/go.md`
  Read: `.claude/rules/.reference/dockerfile/nodejs.md`
  Read: `.claude/rules/.reference/dockerfile/python.md`
  Read: `.claude/rules/.reference/dockerfile/rust.md`
