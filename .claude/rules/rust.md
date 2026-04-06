---
paths:
  - "**/*.rs"
---

* Use full module paths like `std::env::var` instead of `use std::env`
* Follow Rust 2018 Edition module definition conventions
* Use `error::Error` as the standard error type with `error!` and `bail!` macros
* Add `#[tracing::instrument]` to trait impls, public APIs, external I/O, heavy computation, handlers

## Reference

If implementing retryable operations:
  Read: `.claude/rules/.reference/rust/error-handling.md`

If adding tracing instrumentation:
  Read: `.claude/rules/.reference/rust/tracing.md`

If setting up OpenTelemetry in main.rs:
  Read: `.claude/rules/.reference/rust/opentelemetry-setup.md`

If implementing proxy-wasm filters:
  Read: `.claude/rules/.reference/rust/proxy-wasm.md`

If implementing Envoy ext_proc gRPC service:
  Read: `.claude/rules/.reference/rust/ext-proc.md`

If writing eBPF userspace code:
  Read: `.claude/rules/.reference/rust/ebpf.md`
