---
paths:
  - ".github/workflows/**/*.yaml"
---

* Do not add `name` to `run` steps
* Use Docker layer caching with `cache-from` and `cache-to` (see Docker Cache Strategy below)
* Update image digests in Kustomization files after build
* Set `concurrency` with `cancel-in-progress` based on trigger type (see Concurrency below)
* Always set `timeout-minutes` on jobs
* Add `if: github.repository_owner == 'kaidotio'` to jobs that: (1) respond to events triggerable by non-owners (`pull_request`, `issue_comment`, `schedule`, `workflow_run`), or (2) call reusable workflows that depend on self-hosted runners (e.g., `scan` job calling `reusable_scan-image.yaml`). Not needed for `push: branches: main`, `release`, `workflow_dispatch` unless condition (2) applies
* Multi-line shell scripts with pipes: use `set -eo pipefail` (not `-u`, GitHub Actions environment variables are always defined)
* Cross-platform workflows with Windows runners: set `shell: bash` on steps using bash syntax (Windows defaults to PowerShell)
* Use `${{ runner.temp }}` for temporary files needed only during workflow execution; use `~` for persistent home directory references
* Use `sparse-checkout` with the most specific path that covers the files actually used by the workflow (e.g., `app/.github/actions/` not `app/.github/`)

## Docker Cache Strategy

| Build Time | Strategy | cache-to |
|------------|----------|----------|
| Normal (<60 min) | Local only | `type=local,mode=max,dest=...` |
| Long (>60 min) | Local + Registry | `type=local,... --cache-to type=registry,ref=$GHCR_IMAGE:cache,mode=max` |

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
| `issue_comment`, `issues`, `pull_request_review`, `pull_request_review_comment` | No | - | Each event is independent; for `issue_comment`/`issues` `github.ref` is `refs/heads/main` causing unrelated events to share a concurrency group, and for `pull_request_review`/`pull_request_review_comment` each event should complete independently |
| `workflow_run` | No | - | Each reactive action must complete |

## Naming

| Prefix | Purpose |
|--------|---------|
| `00_` | Application Docker builds |
| `10_` | Event handlers (label, comment) |
| `20_` | Release/additional builds (armyknife, taurin, Cloudflare Workers) |
| `40_` | Tests |
| `50_` | Validation |
| `80_` | Metrics/sync |
| `90_` | Auto-generation (readme, crd, jsonnet) |
| `99_` | Snapshots/mirroring |
| `reusable_` | Reusable workflow templates |
| (none) | AI integration (claude*.yaml) |

## Release Workflows

Release workflows (`20_*.yaml`) that build versioned artifacts:

1. Use sparse-checkout with `bin/` included for `bump.sh`
2. Extract version from tag: `VERSION=$(echo ${{ github.event.release.tag_name }} | sed 's/^v//')`
3. Call `bump.sh` before build: `bash ./bin/bump.sh "${VERSION}"`

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
| Local tools (Claude) | `[self-hosted, local]` |
| Standard builds | `ubuntu-24.04` |

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
  Read: `.claude/rules/.reference/.github/workflows/creating-build-workflow.md`
