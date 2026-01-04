---
paths:
  - ".github/workflows/**/*.yaml"
---

* Use Docker layer caching with `cache-from` and `cache-to`
* Update image digests in Kustomization files after build
* Always set `concurrency` with `cancel-in-progress: true`
* Always set `timeout-minutes` on jobs
* Add `if: github.repository_owner == 'kaidotio'` to jobs

## Naming

| Prefix | Purpose |
|--------|---------|
| `00_` | Application Docker builds |
| `10_` | Event handlers (label, comment) |
| `20_` | Release/additional builds (armyknife, taurin) |
| `40_` | Tests |
| `50_` | Validation |
| `80_` | Metrics/sync |
| `90_` | Auto-generation (readme, crd, jsonnet) |
| `99_` | Snapshots/mirroring |
| `reusable_` | Reusable workflow templates |
| (none) | AI integration (claude*.yaml) |

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

## Reference

If creating a new build workflow:
  Read: `.claude/rules/.reference/.github/workflows/creating-build-workflow.md`
