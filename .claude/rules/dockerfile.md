---
paths:
  - "**/Dockerfile*"
---

* Always start with `# syntax=docker/dockerfile:1.4`
* Use multi-stage builds: `builder` stage + runtime stage
* Run as non-root user (UID 65532)
* Use `--mount=type=cache` for package manager caches
* Include `.dockerignore` to exclude unnecessary files

## Reference

If writing a Dockerfile for a specific language:
  Read: `.claude/rules/.reference/dockerfile/go.md`
  Read: `.claude/rules/.reference/dockerfile/python.md`
  Read: `.claude/rules/.reference/dockerfile/rust.md`
