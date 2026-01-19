---
paths:
  - "**/*.go"
---

* Implement tests with [TableDrivenTest](https://go.dev/wiki/TableDrivenTests) pattern for multiple test cases
* Wrap errors with `xerrors.Errorf` to get complete stack traces
* Declare function arguments individually like `(a string, b string)` instead of `(a, b string)`
* Use `Args` struct + `DefaultArgs()` pattern for CLI configuration
* Use one-character receiver names (`c` for `*Client`, `d` for `*Dispatcher`, `h` for `*Handler`)
* Doc comments only for CRD types (controller-gen requirement) and package declarations; omit for regular functions, methods, and non-CRD types

## Logging

| System | When to Use | Examples |
|--------|-------------|----------|
| Standard `log` | Simple HTTP servers, CLI tools, Prometheus exporters | exporter-merger, bakery, github-actions-exporter |
| `log/slog` | Services with OpenTelemetry tracing (traceid/spanid in logs) | github-token-server, kube-crud-server, reporting-server |
| `ctrl.Log` | Controllers and webhooks (controller-runtime) | exactly-one-pod-hook, github-actions-runner-controller |

### Standard `log` package

Only log fatal errors and panics. Do not log informational messages.

| Log | When |
|-----|------|
| `log.Fatal` / `log.Fatalf` | Unrecoverable errors (listen failure, config error, required flag missing) |
| `log.Printf` | Panic recovery only |

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
  Read: `.claude/rules/.reference/go/configuration.md`

If implementing HTTP client:
  Read: `.claude/rules/.reference/go/http-client.md`

If implementing HTTP server:
  Read: `.claude/rules/.reference/go/http-server.md`

If writing tests:
  Read: `.claude/rules/.reference/go/testing.md`

If implementing admission webhook with container injection:
  Read: `.claude/rules/.reference/go/admission-webhook.md`
