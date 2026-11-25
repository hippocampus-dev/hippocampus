# OpenTelemetry Setup Pattern

How to set up OpenTelemetry tracing in Rust applications.

## Feature Flag Pattern

Use empty feature flag with non-optional dependencies:

```toml
[features]
default = ["tracing"]
tracing = []

[dependencies]
opentelemetry = { version = "0.29.0" }
opentelemetry_sdk = { version = "0.29.0" }
opentelemetry-otlp = { version = "0.29.0", default-features = false, features = ["grpc-tonic", "trace", "tls", "tls-roots"] }
tonic = { version = "0.12" }
tracing = { version = "0.1.37" }
tracing-opentelemetry = { version = "0.30.0" }
tracing-subscriber = { version = "0.3.16", features = ["json", "env-filter"] }
```

Do NOT use `optional = true` for these dependencies.
Name `tonic` at the 0.12 that `opentelemetry-otlp` links, renaming it (`tonic-otlp = { version = "0.12", package = "tonic" }` in `cluster/applications/envoy-markdownify/Cargo.toml`) where the crate already depends on another `tonic` major.

## cfg!() vs #[cfg()]

| Syntax | Type | Use Case |
|--------|------|----------|
| `cfg!(feature = "...")` | Runtime check | Feature-gated initialization logic |
| `#[cfg(feature = "...")]` | Compile-time exclusion | Conditional struct fields, allocators |

Use `cfg!()` for tracing initialization.
Use `#[cfg()]` only for:
- Global allocator selection
- Conditional struct definitions
- Platform-specific code

```rust
// Good: Runtime check for tracing setup
let tracer_provider = if cfg!(feature = "tracing") {
    let exporter = opentelemetry_otlp::SpanExporter::builder()...
    Some(tracer_provider)
} else {
    None
};

// Good: Compile-time for allocator (cannot be runtime)
#[cfg(feature = "jemalloc")]
#[global_allocator]
static GLOBAL: tikv_jemallocator::Jemalloc = tikv_jemallocator::Jemalloc;
```

## Main Function Pattern

```rust
type Return = Result<(), Box<dyn std::error::Error + Send + Sync + 'static>>;

fn main() -> Return {
    let args: Args = Args::parse();

    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()?
        .block_on(async {
            opentelemetry::global::set_text_map_propagator(
                opentelemetry_sdk::propagation::TraceContextPropagator::new(),
            );

            let tracer_provider = if cfg!(feature = "tracing") {
                // OpenTelemetry setup here
                Some(tracer_provider)
            } else {
                None
            };

            let result = process(args).await;

            if let Some(tracer_provider) = tracer_provider {
                // opentelemetry_sdk already logs a failed final export as BatchSpanProcessor.ExportError
                let _ = tracer_provider.shutdown();
            }

            result
        })?;

    Ok(())
}
```

Shut the provider down inside `block_on`, since the batch processor's final export goes through the tonic channel whose worker task dies with the runtime.

## Tracer Setup

```rust
let tracer_provider = if cfg!(feature = "tracing") {
    let exporter = opentelemetry_otlp::SpanExporter::builder()
        .with_tonic()
        .with_tls_config(tonic::transport::ClientTlsConfig::new().with_native_roots())
        .build()?;
    let tracer_provider = opentelemetry_sdk::trace::SdkTracerProvider::builder()
        .with_batch_exporter(exporter)
        .with_resource(
            opentelemetry_sdk::Resource::builder_empty()
                .with_detectors(&[
                    Box::new(opentelemetry_sdk::resource::EnvResourceDetector::new()),
                    Box::new(opentelemetry_sdk::resource::SdkProvidedResourceDetector),
                ])
                .build(),
        )
        .build();
    let telemetry = tracing_opentelemetry::layer()
        .with_tracer(tracer_provider.tracer(env!("CARGO_PKG_NAME")));

    let formatter = tracing_subscriber::fmt::format().with_source_location(true);
    #[cfg(not(debug_assertions))]
    let formatter = formatter
        .json()
        .with_current_span(false)
        .with_span_list(false)
        .flatten_event(true);
    let fmt = tracing_subscriber::fmt::layer().event_format(formatter);

    tracing_subscriber::registry()
        .with(fmt)
        .with(telemetry)
        .with(tracing_subscriber::EnvFilter::from_default_env())
        .init();

    Some(tracer_provider)
} else {
    None
};
```

Pass `with_tls_config` even when the endpoint is plaintext, since tonic 0.12 fails an `https://` endpoint that has no TLS config with `HttpsUriWithoutTlsSupport` and applies the config only to `https://` URIs, and run the binary on an image that ships CA certificates, since `with_native_roots` fails the exporter build with `NativeCertsNotFound` without them.

Attach `EnvFilter` to the registry rather than to the fmt layer with `.with_filter()`.
A per-layer filter leaves the telemetry layer unfiltered, and `tracing-opentelemetry`'s layer overrides neither `enabled` nor `register_callsite`, so the subscriber reports every callsite as enabled and the `tracing::enabled!` guard inside the `opentelemetry_tracing` macros stops gating anything.

## Example

Copy from: `cluster/applications/rust_de_llama/src/main.rs`
