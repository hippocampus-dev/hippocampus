---
paths:
  - "**/*.rs"
---

* Use full module paths like `std::env::var` instead of `use std::env`, except for a trait that must be in scope for method resolution (`use clap::Parser;` for `Args::parse()`) - such an import is load-bearing and deleting it breaks the build
* Follow Rust 2018 Edition module definition conventions
* Use `error::Error` as the standard error type with `error!` and `bail!` macros in a crate that already depends on `packages/error` - an eBPF loader returns `Box<dyn std::error::Error + Send + Sync + 'static>` instead, so adding the dependency to one makes it the outlier among its siblings
* Add `#[tracing::instrument]` to trait impls, public APIs, external I/O, heavy computation, handlers - an eBPF loader declares no `tracing` at all and reports through `println!`/`eprintln!`, so leave one uninstrumented rather than pulling the dependency in
* Parse CLI arguments with a `#[derive(clap::Parser, Debug)] pub struct Args` bound as `let args: Args = Args::parse();`, spelling attributes `#[clap(...)]` rather than clap 4's `#[arg]`/`#[command]`, typing path-valued options `std::path::PathBuf`, and giving a `default_value` to every field whose type does not already make it optional (`Option<T>`, `bool`, `Vec<T>`) - a `#[clap(subcommand)]` is the only mandatory input any binary here takes, so a mandatory flag would make that binary the sole exception
* Decode a fixed-layout byte buffer (BPF perf event, FUSE wire struct) through `unsafe impl plain::Plain for T` plus `plain::copy_from_bytes`, never a hand-rolled `read_unaligned` or per-field `from_ne_bytes` - reach for `plain::as_bytes` for the reverse direction, and never `plain::from_bytes`, which calls `check_alignment` and so rejects a buffer whose address a `read(2)` happened to leave unaligned

## Reference

If implementing retryable operations:
  Read: `.claude/reference/rust/error-handling.md`

If adding tracing instrumentation:
  Read: `.claude/reference/rust/tracing.md`

If setting up OpenTelemetry in main.rs:
  Read: `.claude/reference/rust/opentelemetry-setup.md`

If writing tests:
  Read: `.claude/reference/rust/testing.md`

If implementing proxy-wasm filters:
  Read: `.claude/reference/rust/proxy-wasm.md`

If implementing Envoy ext_proc gRPC service:
  Read: `.claude/reference/rust/ext-proc.md`

If writing eBPF userspace code:
  Read: `.claude/reference/rust/ebpf.md`
