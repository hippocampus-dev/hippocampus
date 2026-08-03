---
paths:
  - "**/*.go"
---

* Implement tests with [TableDrivenTest](https://go.dev/wiki/TableDrivenTests) pattern for multiple test cases
* Wrap errors with `xerrors.Errorf` to get complete stack traces
* Declare function arguments individually like `(a string, b string)` instead of `(a, b string)`
* Use `Args` struct + `DefaultArgs()` for Cobra apps, `envOrDefaultValue` for flag-based apps
* Use one-character receiver names (`c` for `*Client`, `d` for `*Dispatcher`, `h` for `*Handler`)
* Doc comments only for CRD types (controller-gen requirement) and package declarations; omit for regular functions, methods, and non-CRD types

## Logging

| System | When to Use | Examples |
|--------|-------------|----------|
| Standard `log` | Simple HTTP servers, CLI tools, Prometheus exporters | exporter-merger, bakery, github-actions-exporter |
| `log/slog` | Services with OpenTelemetry tracing (traceid/spanid in logs) | github-token-server, kube-crud-server, reporting-server |
| `ctrl.Log` | Controllers and webhooks (controller-runtime) | exactly-one-pod-hook, github-actions-runner-controller |

### Standard `log` package

Only log errors and panics.
Do not log informational messages.

| Log | When |
|-----|------|
| `log.Fatal` / `log.Fatalf` | Unrecoverable errors (listen failure, config error, required flag missing) |
| `log.Printf` | Recoverable errors in handlers, and panic recovery |

### `log/slog` package

Use with OpenTelemetry for structured logging with trace context.

| Level | When |
|-------|------|
| `slog.Error` | Recoverable errors in handlers |
| `slog.Warn` | Unexpected but handled conditions |
| `slog.Info` | Business events (only for logging services like reporting-server) |
| `slog.Debug` | Expected conditions (e.g., client closed connection) |

### Do NOT log

- Startup messages (e.g., `log.Printf("server started on %s", addr)`)
- Successful operations
- Routine request handling (use HTTP status codes or return errors)

## Reference

If implementing CLI configuration:
  Read: `.claude/reference/go/configuration.md`

If implementing HTTP client:
  Read: `.claude/reference/go/http-client.md`

If implementing HTTP server in a service (long-running, Kubernetes-deployed):
  Read: `.claude/reference/go/http-server.md`

If implementing HTTP server in a CLI tool (short-lived, one-shot callback):
  Use `go http.Serve(listener, handler)` directly — graceful shutdown is not needed, process exit reclaims resources (see `internal/bakery/bakery.go`, `internal/oauth/pkce.go`)

If writing tests:
  Read: `.claude/reference/go/testing.md`

If implementing admission webhook with container injection:
  Read: `.claude/reference/go/admission-webhook.md`

If implementing CloudEvents handler that returns a reply event or forwards to an HTTP sink, or emitting events into a pipeline that deduplicates them:
  Read: `.claude/reference/go/cloudevents-handler.md`

If constructing an Alertmanager alert payload from an application (`AlertmanagerAlert.Labels`, `AlertmanagerAlert.Annotations`):
  Read: `.claude/rules/cluster/manifests/alerts.md`

If checking whether a GitHub issue already exists immediately before creating one:
  Match on title over `Issues.ListByRepo`, skipping `IsPullRequest()` entries since the endpoint returns pull requests too — `Search.Issues` draws on a quota separate from the core limit and reads an asynchronous index that omits issues created seconds earlier, and a `Labels` filter never matches issues whose labels `Issues.Create` silently dropped for lack of push access (see `cluster/applications/alerthandler/handler/critical_alert_handler.go`)
