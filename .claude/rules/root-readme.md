---
paths:
  - "README.md"
---

* Update Project Structure counts and tree when directories are added, removed, or restructured, and when files are added or removed under a tree block that enumerates files individually
* Add workflow badges for new `.github/workflows/*.yaml` files
* Title is the capitalized project name (`# Hippocampus`), followed by the logo image and a free-form one-line description opening with that same name
* The `<!-- TOC -->` pair sits after the badge block, not directly after the title
* Project Structure tree entries are grouped by category, so append new entries to their existing category run instead of re-sorting the tree

## Project Structure Counts

| Item | Command | Location in README |
|------|---------|-------------------|
| Packages | `ls -d packages/*/ \| wc -l` | `packages/` line |
| Applications | `ls -d cluster/applications/*/ \| wc -l` | `cluster/applications/` line |
| Manifests | `ls -d cluster/manifests/*/ \| wc -l` | `cluster/manifests/` line |
| Application Manifests | `comm -12 <(ls -d cluster/applications/*/ \| xargs -n1 basename \| sort) <(ls -d cluster/manifests/*/ \| xargs -n1 basename \| sort) \| wc -l` | `application manifests` line |
| Docker Compose Services | `sed -n '/^services:/,/^networks:/p' docker-compose.yaml \| grep -c '^  [a-z]'` | root `docker-compose.yaml` line |

Docker Compose Services counts services declared in the root file only; profile services pulled in by `include:` are excluded.

Explicit `cluster/manifests/` tree entries plus the `application manifests` and `application variants` lines must total the Manifests count.
Variants are `cluster/manifests/` directories with no same-named `cluster/applications/` entry that still deploy a repository application; resolve the source application from the `00_*` workflow (`KUSTOMIZATION` and `on.push.paths`), or from a `resources:` reference into `cluster/manifests/utilities/`.

## Workflow Badges

Format: `[![{name}](https://github.com/hippocampus-dev/hippocampus/actions/workflows/{prefix}_{name}.yaml/badge.svg)](https://github.com/hippocampus-dev/hippocampus/actions/workflows/{prefix}_{name}.yaml)`

| Source | Badge Name |
|--------|------------|
| `{prefix}_{name}.yaml` | Use `{name}` (without prefix) |

Badges are ordered by workflow prefix (00_, 10_, 20_, ..., 99_), then alphabetically within each prefix group.

## Application Categories

When adding new applications to Project Structure, use these category prefixes:

| Category | When to Use | Examples |
|----------|-------------|----------|
| AI/ML: | AI services, embeddings, language models | embedding-gateway, whisper-worker |
| Alerting: | Alert processing and forwarding | alerthandler |
| Controller: | Kubernetes controllers with reconciliation loop | grafana-manifest-controller, nodeport-controller |
| DevTool: | Developer tooling and automation | bakery, chrome-devtools-mcp, playwright-mcp |
| Logging: | Log collection and aggregation | fluentd-aggregator, slack-logger |
| Monitoring: | Metrics exporters, observability | connectracer, exporter-merger |
| Proxy: | HTTP/TCP proxies, protocol adapters | anonymous-proxy, tcp-proxy |
| Utility: | General utilities, infrastructure | endpoint-broadcaster, token-request-server |
| Web: | Web applications, dashboards | csviewer, kube-crud |
| Webhook: | Kubernetes admission webhooks | exactly-one-pod-hook, statefulset-hook |

Determine category by reading the application's `README.md` or source code.
