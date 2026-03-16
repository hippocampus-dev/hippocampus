# OpenTelemetry Setup Pattern

How to set up OpenTelemetry tracing in Rust applications.

## Feature Flag Pattern

Use empty feature flag with non-optional dependencies:

```toml
[features]
default = ["tracing"]
tracing = []

[dependencies]
opentelemetry = { version = "0.18.0", features = ["rt-tokio"] }
opentelemetry-otlp = { version = "0.11.0", features = ["tls", "tls-roots"] }
tracing = { version = "0.1.37" }
tracing-opentelemetry = { version = "0.18.0" }
tracing-subscriber = { version = "0.3.16", features = ["json", "env-filter"] }
```

Do NOT use `optional = true` for these dependencies.

## cfg!() vs #[cfg()]

| Syntax | Type | Use Case |
|--------|------|----------|
| `cfg!(feature = "...")` | Runtime check | Feature-gated initialization logic |
| `#[cfg(feature = "...")]` | Compile-time exclusion | Conditional struct fields, allocators |

Use `cfg!()` for tracing initialization. Use `#[cfg()]` only for:
- Global allocator selection
- Conditional struct definitions
- Platform-specific code

```rust
// Good: Runtime check for tracing setup
if cfg!(feature = "tracing") {
    let tracer = opentelemetry_otlp::new_pipeline()...
}

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
                opentelemetry::sdk::propagation::TraceContextPropagator::new(),
            );

            if cfg!(feature = "tracing") {
                // OpenTelemetry setup here
            }

            process(args).await
        })?;

    if cfg!(feature = "tracing") {
        opentelemetry::global::shutdown_tracer_provider();
    }

    Ok(())
}
```

## Tracer Setup

```rust
if cfg!(feature = "tracing") {
    let tracer = opentelemetry_otlp::new_pipeline()
        .tracing()
        .with_exporter(opentelemetry_otlp::new_exporter().tonic().with_env())
        .with_trace_config(opentelemetry::sdk::trace::config().with_resource(
            opentelemetry::sdk::Resource::from_detectors(
                std::time::Duration::from_secs(0),
                vec![
                    Box::new(opentelemetry::sdk::resource::EnvResourceDetector::new()),
                    Box::new(opentelemetry::sdk::resource::SdkProvidedResourceDetector),
                ],
            ),
        ))
        .install_batch(opentelemetry::runtime::Tokio)?;
    let telemetry = tracing_opentelemetry::layer().with_tracer(tracer);

    let formatter = tracing_subscriber::fmt::format().with_source_location(true);
    #[cfg(not(debug_assertions))]
    let formatter = formatter
        .json()
        .with_current_span(false)
        .with_span_list(false)
        .flatten_event(true);
    let fmt = tracing_subscriber::fmt::layer()
        .event_format(formatter)
        .with_filter(tracing_subscriber::EnvFilter::from_default_env());

    tracing_subscriber::registry()
        .with(fmt)
        .with(telemetry)
        .init();
}
```

## Example

Copy from: `cluster/applications/rust_de_llama/src/main.rs`
