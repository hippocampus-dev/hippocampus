---
paths:
  - "README.md"
---

* Update Project Structure counts and tree when directories are added, removed, or restructured
* Add workflow badges for new `.github/workflows/00_*.yaml` files
* Maintain alphabetical ordering within each section

## Project Structure Counts

| Item | Command | Location in README |
|------|---------|-------------------|
| Packages | `ls -d packages/*/ \| wc -l` | `packages/` line |
| Applications | `ls -d cluster/applications/*/ \| wc -l` | `cluster/applications/` line |
| Manifests | `ls -d cluster/manifests/*/ \| wc -l` | `cluster/manifests/` line |
| Application Manifests | `comm -12 <(ls cluster/applications \| sort) <(ls cluster/manifests \| sort) \| wc -l` | `application manifests` line |

## Workflow Badges

Format: `[![{name}](https://github.com/hippocampus-dev/hippocampus/actions/workflows/00_{name}.yaml/badge.svg)](https://github.com/hippocampus-dev/hippocampus/actions/workflows/00_{name}.yaml)`

| Source | Badge Name |
|--------|------------|
| `00_{name}.yaml` | Use `{name}` (without `00_` prefix) |

Badges are ordered alphabetically by name.

## Application Categories

When adding new applications to Project Structure, use these category prefixes:

| Category | When to Use | Examples |
|----------|-------------|----------|
| (AI/ML) | AI services, embeddings, language models | embedding-gateway, whisper-worker |
| (Alerting) | Alert processing and forwarding | alerthandler |
| (Controller) | Kubernetes controllers with reconciliation loop | grafana-manifest-controller, nodeport-controller |
| (DevTool) | Developer tooling, build automation | bakery, chrome-devtools-mcp, playwright-mcp |
| (Logging) | Log collection and aggregation | fluentd-aggregator, slack-logger |
| (Monitoring) | Metrics exporters, observability | connectracer, exporter-merger |
| (Proxy) | HTTP/TCP proxies, protocol adapters | anonymous-proxy, tcp-proxy |
| (Utility) | General utilities, infrastructure | endpoint-broadcaster, token-request-server |
| (Web) | Web applications, dashboards | csviewer, kube-crud |
| (Webhook) | Kubernetes admission webhooks | exactly-one-pod-hook, statefulset-hook |

Determine category by reading the application's `CLAUDE.md` or source code.
