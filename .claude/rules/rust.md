---
paths:
  - "**/*.rs"
---

* Use full module paths like `std::env::var` instead of `use std::env`, except for a trait that must be in scope for method resolution (`use clap::Parser;` for `Args::parse()`) - such an import is load-bearing and deleting it breaks the build
* Follow Rust 2018 Edition module definition conventions
* Use `error::Error` as the standard error type with `error!` and `bail!` macros in a crate that already depends on `packages/error`, and `Box<dyn std::error::Error + Send + Sync + 'static>` throughout one that does not - taking on the dependency is a separate decision from following this rule
* Add `#[tracing::instrument]` to trait impls, public APIs, external I/O, heavy computation, handlers of a crate that declares `tracing` - leave a crate that does not declare it uninstrumented rather than pulling the dependency in for one call site
* Emit every log event through the matching `opentelemetry_tracing` macro in a crate that depends on `packages/opentelemetry-tracing`, never the bare `tracing::` one - the `event!` macro behind them injects the current span's `traceid` and `spanid` as event fields and a bare macro omits both while still compiling and logging, and it is `traceid` that the `derivedFields` matcher of the Loki datasource in `cluster/manifests/grafana/overlays/dev/files/datasource.yaml` turns into a link from the log line to its trace in Tempo; a crate without that dependency logs bare rather than taking it on for its log lines
* Keep `tracing-opentelemetry` and `opentelemetry` at the same version in `packages/opentelemetry-tracing/Cargo.toml` and in every application that registers the layer - each `cluster/applications/*` resolves its own `Cargo.lock`, so raising one side alone links both versions and `OpenTelemetrySpanExt::context()` looks for a layer the subscriber never registered, stamping nothing onto the event instead of failing
* Parse CLI arguments with a `#[derive(clap::Parser, Debug)] pub struct Args` bound as `let args: Args = Args::parse();` in a `main` that spells its `Result` as a `type Return = Result<(), E>` alias with `E` chosen per the `error::Error` bullet, spelling attributes `#[clap(...)]` rather than clap 4's `#[arg]`/`#[command]`, writing help text as `///` doc comments rather than `help = "..."`, and typing path-valued options `std::path::PathBuf`, giving each subcommand its own module with its own `Args` and a `run(args: Args, ...)` that a `#[derive(clap::Parser, Debug)] enum SubCommand` in `main.rs` dispatches to (`packages/teleput/src/main.rs`)
* Give every CLI input that has a sensible default a flag with a default - `default_value_t` for a numeric type, `default_value` for a `String` or `PathBuf` - unless its type already makes it optional (`Option<T>`, `bool`, `Vec<T>`), never putting one on an `Option<T>`, which clap then never leaves `None`, and take an operand no default fits as a positional argument instead (`#[clap(name = "FILE")]` in `packages/hippocampus-standalone/src/index.rs`), typed so that it is required only where the command cannot run without it (`address: String` in `packages/teleput/src/client.rs`) - no binary here takes a mandatory flag, so introducing one makes that binary the sole exception
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

If writing the userspace daemon behind a fuse-bpf mount:
  Read: `.claude/reference/bpf/fuse-bpf.md`
