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
* Use `xvfb-run -a` (`--auto-servernum`), never bare `xvfb-run`, in any ENTRYPOINT — the bare form always binds server number `:99` and leaves `/tmp/.X99-lock`/`/tmp/.X11-unix/X99` behind on an unclean exit; since a Pod's `/tmp` is typically a Memory-backed `emptyDir` that persists across container restarts within the same Pod (see `cluster/manifests.md` Container Defaults), the stale lock then makes every subsequent restart fail immediately with "Xvfb failed to start", a permanent CrashLoopBackOff (example: `cluster/applications/snapshot-controller/Dockerfile`)

## Language Version Consistency

The builder image version in Dockerfile must match the language version in the project's dependency file.
When changing either, update both.

| Language | Dockerfile | Dependency file and key | How the two spellings differ |
|----------|-----------|-------------------------|------------------------------|
| Go | `golang:{major}.{minor}-bookworm` | `go.mod` `go` directive | the directive adds a patch component the tag omits |
| Python | `python:{major}.{minor}-slim-bookworm` | `pyproject.toml` `requires-python` | the key is `>=` the tag's version |
| Rust | `rust:{major}.{minor}-bookworm` | `rust-toolchain.toml` `channel` | the channel adds a patch component the tag omits |
| Node.js | `node:{major}-bookworm-slim` | `package.json` `engines.node` | the key is `>=` the tag's version |

## Reference

If writing a Dockerfile for a specific language:
  Read: `.claude/reference/dockerfile/go.md`
  Read: `.claude/reference/dockerfile/nodejs.md`
  Read: `.claude/reference/dockerfile/python.md`
  Read: `.claude/reference/dockerfile/rust.md`
