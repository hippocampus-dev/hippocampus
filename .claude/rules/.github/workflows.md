---
paths:
  - ".github/workflows/**/*.yaml"
---

* Do not add `name` to `run` steps
* Use Docker layer caching with `cache-from` and `cache-to` (see Docker Cache Strategy below)
* Update image digests in Kustomization files after build
* Set `concurrency` with `cancel-in-progress` based on trigger type (see Concurrency below)
* Always set `timeout-minutes` on jobs
* Add `if: github.repository_owner == 'kaidotio'` to jobs that: (1) respond to events triggerable by non-owners (`pull_request`, `issue_comment`, `schedule`, `workflow_run`), or (2) call reusable workflows that depend on self-hosted runners (e.g., `scan` job calling `reusable_scan-image.yaml`). Not needed for `push: branches: main`, `release`, `workflow_dispatch` unless condition (2) applies. Release workflows (`20_*`) must not adopt pattern (2) so they can run on the public mirror (see Release Workflows)
* Jobs depending on `runner` reusable workflow must use `!cancelled()` in their `if:` condition to handle partial failures (e.g., token server unavailable). See `.claude/reference/.github/workflows/creating-build-workflow.md` for exact patterns
* Workflows calling `reusable_dynamic-runner.yaml` (or any workflow whose chain includes `reusable_issue-github-token.yaml`) must declare `id-token: write` in the top-level `permissions` block; GitHub Actions requires caller permissions to be a superset of the reusable workflow's permissions, otherwise the run fails with `startup_failure`
* Always derive a git branch name from `env.IMAGE` as `branch="${IMAGE//\//-}"`, never `branch="${IMAGE}"` (the substitution is a no-op when IMAGE has no slash). Git cannot create `refs/heads/foo` when `refs/heads/foo/bar` already exists; IMAGE values with slashes (e.g., `snapshot-controller/snapshot-diff-server`) cause a ref namespace collision that silently breaks the deploy step
* Shell code in `run` steps follows bash conventions from `.claude/rules/bash.md` (variable quoting, conditionals, pipefail patterns)
* Multi-line shell scripts: use `set -eo pipefail` (not `-u`, GitHub Actions environment variables are always defined)
* Cross-platform workflows with Windows runners: set `shell: bash` on steps using bash syntax (Windows defaults to PowerShell)
* Use `${{ runner.temp }}` for temporary files needed only during workflow execution; use `~` for persistent home directory references
* Use `sparse-checkout` with the most specific path that covers the files actually used by the workflow (e.g., `app/.github/actions/` not `app/.github/`). When specifying individual file paths (not directories), add `sparse-checkout-cone-mode: false` — cone mode (the default) rejects file-level paths
* Before `actions/checkout`, add a conditional step `echo "GIT_CONFIG_GLOBAL=${{ runner.temp }}/gitconfig" >> $GITHUB_ENV` that runs only on self-hosted local runners (`if: contains(needs.runner.outputs.runner, 'local')`), followed by `git config --global submodule.recurse false` as a separate step. This ensures `actions/checkout`'s internal `git config --global --add safe.directory` writes to the temp file instead of `~/.gitconfig` (self-hosted local runners need an explicit path; empty `GIT_CONFIG_GLOBAL` causes `git config --global` to fail on GitHub-hosted runners)
* Before `uses: ./.github/actions/setup-docker` or `docker login`, add a conditional step `echo "DOCKER_CONFIG=${{ runner.temp }}/.docker" >> $GITHUB_ENV` that runs only on self-hosted local runners (`if: contains(needs.runner.outputs.runner, 'local')` in regular workflows, or `if: contains(inputs.runner, 'local')` / `if: contains(inputs.runs-on, 'local')` in reusable workflows). This prevents `docker login` from overwriting `~/.docker/config.json` with short-lived `ghs_` GitHub App installation tokens
* When using `bash .github/scripts/cleanup.sh` to free disk space for large builds, add `if: ${{ !contains(needs.runner.outputs.runner, 'local') }}` to skip it on self-hosted local runners. The script removes system directories (`/usr/lib/gcc`, `/usr/lib/llvm-*`, etc.) that are needed by the host system on persistent local runners but are safe to remove on ephemeral GitHub-hosted runners. Do not use `cleanup.sh` in workflows that install Python packages with C extensions (e.g., `aiohttp`) — the script removes `/usr/lib/gcc` headers required for compilation even on GitHub-hosted runners
* When a workflow uses `uv sync`/`uv run` with `actions/setup-python`, set `python-version` to match the project's `requires-python` lower bound in `pyproject.toml` and add `env: UV_PYTHON_DOWNLOADS: never` to the uv step. Without this, a version mismatch causes uv to silently download a standalone Python interpreter that lacks the system C headers needed by packages with C extensions

## Docker Cache Strategy

| Build Time | Strategy | cache-to |
|------------|----------|----------|
| Normal (<60 min) | Local only | `type=local,mode=max,dest=...` |
| Long (>60 min) | Local + Registry | `type=local,... --cache-to type=registry,ref=$GHCR_IMAGE:cache,mode=max` |

Local cache is stored in a per-image directory (`docker-build/{IMAGE}`) to prevent cache collisions between parallel builds on shared runners.

Registry cache uses `:cache` tag (not `:main`) to avoid overwriting the production image. Add registry cache when:
- Build exceeds 60 minutes (GitHub Actions cache may be evicted before next build)
- Dockerfile changes infrequently (Rust cross-compilation base images)

## Concurrency

| Trigger | `concurrency` | `cancel-in-progress` | Reason |
|---------|---------------|----------------------|--------|
| `push`, `pull_request`, `release` | Yes | `true` | Newer commit supersedes older |
| `workflow_dispatch` | Yes | `true` | User can re-trigger |
| `push` + `schedule` (mixed) | Yes | `true` | Push is primary trigger |
| `schedule` only | Yes | `false` | Current run should complete |
| `schedule` + `workflow_dispatch` | Yes | `false` | Schedule is primary trigger; workflow_dispatch is manual re-trigger |
| `issue_comment`, `issues`, `pull_request_review`, `pull_request_review_comment` | No | - | Each event is independent; for `issue_comment`/`issues` `github.ref` is `refs/heads/main` causing unrelated events to share a concurrency group, and for `pull_request_review`/`pull_request_review_comment` each event should complete independently |
| `workflow_run` | Yes | `false` | Each reactive action must complete |

## Naming

| Prefix | Purpose |
|--------|---------|
| `00_` | Application Docker builds |
| `10_` | Event handlers (label, comment) |
| `20_` | Release/additional builds (armyknife, taurin, Cloudflare Workers) |
| `40_` | Tests |
| `50_` | Validation |
| `80_` | Metrics/sync |
| `90_` | Auto-generation (readme, crd, jsonnet), snapshots |
| `99_` | Mirroring |
| `reusable_` | Reusable workflow templates |
| (none) | AI integration (claude*.yaml) |

## Release Workflows

Release workflows (`20_*.yaml`) that build versioned artifacts:

1. Use sparse-checkout with `bin/` included for `bump.sh`
2. Extract version from tag: `VERSION=$(echo ${{ github.event.release.tag_name }} | sed 's/^v//')`
3. Call `bump.sh` before build: `bash ./bin/bump.sh "${VERSION}"`
4. Use the direct-runner pattern (`runs-on: ubuntu-24.04`, or a platform matrix for cross-OS artifacts) without an `if: github.repository_owner` guard and without the dynamic-runner pattern. Release workflows must run on the public mirror (`hippocampus-dev/hippocampus`), which has no self-hosted runners: release assets are published to the firing repository's release and pulled by unauthenticated clients from the public mirror (e.g., `remotty`'s `Dockerfile` and the `github-actions-runner-controller` runner image `ADD https://github.com/hippocampus-dev/hippocampus/releases/download/...`). The dynamic-runner `check` job is pinned to `[self-hosted, local]` and would hang there, while an owner guard would skip the job entirely, leaving the public release without those binaries. See `.claude/reference/.github/workflows/creating-build-workflow.md` for the exact step patterns.
Use `bash` prefix because `bump.sh` lacks execute permission in the repository.

`bump.sh` handles sparse-checkout gracefully - uses `find` which ignores missing directories, and `if [ -f ... ]` checks for specific files.

## PR Trigger

```yaml
on:
  pull_request:
    types: [opened, edited, reopened, synchronize, ready_for_review]
```

Exclude drafts: `github.event.pull_request.draft == false && github.event.pull_request.state == 'open'`

## Runner Selection

| Condition | Runner |
|-----------|--------|
| Cluster internal access | `[self-hosted, github-actions-runner-controller]` |
| Local tools (Claude) only | `[self-hosted, local]` |
| Deploy workflows `00_*` (prefer local, fall back to hosted) | `reusable_dynamic-runner.yaml` with `labels: self-hosted,local`, `fallback-runner: ubuntu-24.04` |
| Release workflows `20_*` (must run on the public mirror) | `ubuntu-24.04` (no owner guard, no dynamic-runner) |
| Standard builds | `ubuntu-24.04` |

When installing binaries at runtime (e.g., `goreleaser`, `cross`), extract to `${{ runner.temp }}/bin` and append to `$GITHUB_PATH` (`echo "${{ runner.temp }}/bin" >> "$GITHUB_PATH"`) rather than `/usr/local/bin`. `github-actions-runner-controller` runners have a read-only root filesystem, and self-hosted local runners would retain system-wide binaries across runs; `${{ runner.temp }}` is writable on all runner types and auto-cleans per job.

## Path Exclusions

Exclude non-source files from build triggers:

```yaml
paths:
  - "!**/*.md"
  - "!**/manifests/**"
  - "!**/skaffold/**"
```

## Secrets

Secrets are managed via `bin/repository-settings.sh`. Add new secrets to the appropriate array.

| Scope | Array | `environment` required |
|-------|-------|------------------------|
| All workflows | `repository_secrets` | No |
| Protected deployments | `environment_secrets` | `deployment` |

Jobs that access `environment_secrets` (e.g., `GPG_PASSPHRASE`) must set `environment: deployment`. This includes external service deployments (Cloudflare, Tauri signing) and scan/token-issuing jobs that decrypt GPG-encrypted tokens.

## Reference

If creating a new build workflow:
  Read: `.claude/reference/.github/workflows/creating-build-workflow.md`
