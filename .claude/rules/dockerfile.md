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

## Language Version Consistency

The builder image version in Dockerfile must match the language version in the project's dependency file. When changing either, update both.

| Language | Dockerfile | Dependency file | Example |
|----------|-----------|-----------------|---------|
| Go | `golang:1.25-bookworm` | `go.mod` `go 1.25.0` | `1.25` ↔ `1.25.0` |
| Python | `python:3.14-slim-bookworm` | `pyproject.toml` `requires-python` | `3.14` ↔ `>=3.14` |
| Rust | `rust:1.89-bookworm` | `rust-toolchain.toml` | `1.89` ↔ `1.89.0` |
| Node.js | `node:22-bookworm-slim` | `package.json` `engines.node` | `22` ↔ `>=22` |

## Reference

If writing a Dockerfile for a specific language:
  Read: `.claude/reference/dockerfile/go.md`
  Read: `.claude/reference/dockerfile/nodejs.md`
  Read: `.claude/reference/dockerfile/python.md`
  Read: `.claude/reference/dockerfile/rust.md`
