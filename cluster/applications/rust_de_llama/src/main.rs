use clap::Parser;
use opentelemetry::metrics::MeterProvider;
use opentelemetry_otlp::WithExportConfig;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, Layer};

mod config;
mod handler;
mod middleware;
mod model_manager;
mod parallel;

#[cfg(not(feature = "jemalloc"))]
#[global_allocator]
static GLOBAL: std::alloc::System = std::alloc::System;

#[cfg(feature = "jemalloc")]
#[global_allocator]
static GLOBAL: tikv_jemallocator::Jemalloc = tikv_jemallocator::Jemalloc;

type Return = Result<(), Box<dyn std::error::Error + Send + Sync + 'static>>;

#[derive(clap::Parser, Debug)]
struct Args {
    #[clap(long, default_value = "127.0.0.1:8080")]
    address: String,
    #[clap(long = "monitor-address", default_value = "127.0.0.1:8081")]
    monitor_address: String,

    #[clap(long, default_value_t = 1)]
    lameduck: u64,

    #[clap(long = "http-keepalive", default_value_t = true)]
    http_keepalive: bool,

    #[clap(long = "model-directory", default_value = "models")]
    model_directory: String,

    #[clap(long = "max-blocking-threads")]
    max_blocking_threads: Option<usize>,

    #[clap(
        long = "n-ctx",
        default_value_t = 0,
        help = "Context size (0 = auto-detect from model)"
    )]
    n_ctx: i32,

    #[clap(long = "n-parallel", default_value_t = 4)]
    n_parallel: usize,

    #[clap(long = "n-batch", default_value_t = 512)]
    n_batch: i32,

    #[clap(long = "n-ubatch", default_value_t = 512)]
    n_ubatch: i32,

    #[clap(
        long = "preload-model",
        help = "Model file to preload on startup for better first request performance (e.g., llama-2-7b.Q4_K_M.gguf)"
    )]
    preload_model: Option<String>,
}

fn main() -> Return {
    #[cfg(debug_assertions)]
    dotenv::dotenv()?;

    let args: Args = Args::parse();

    let worker_threads = std::thread::available_parallelism()
        .map(|n| n.get())
        .unwrap_or(8);

    let max_blocking_threads = args
        .max_blocking_threads
        .unwrap_or_else(|| std::cmp::min(4, worker_threads / 8).max(2));

    tokio::runtime::Builder::new_multi_thread()
        .worker_threads(worker_threads)
        .max_blocking_threads(max_blocking_threads)
        .enable_all()
        .build()?
        .block_on(async {
            opentelemetry::global::set_text_map_propagator(
                opentelemetry::sdk::propagation::TraceContextPropagator::new(),
            );

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

            process(args).await
        })?;

    if cfg!(feature = "tracing") {
        opentelemetry::global::shutdown_tracer_provider();
    }

    Ok(())
}

#[derive(Clone, Debug)]
pub struct MonitorState {
    exporter: opentelemetry_prometheus::PrometheusExporter,
}

pub type LlamaBackend = std::sync::Arc<model_manager::ModelManager>;

#[derive(Clone)]
pub struct AppState {
    pub llama_backend: LlamaBackend,
    pub processed_tokens_counter: opentelemetry::metrics::Counter<u64>,
}

async fn process(args: Args) -> Return {
    let exporter = opentelemetry_prometheus::exporter(
        opentelemetry::sdk::metrics::controllers::basic(
            opentelemetry::sdk::metrics::processors::factory(
                opentelemetry::sdk::metrics::selectors::simple::inexpensive(),
                opentelemetry::sdk::export::metrics::aggregation::cumulative_temporality_selector(),
            )
            .with_memory(true),
        )
        .build(),
    )
    .init();

    let meter = exporter.meter_provider()?.meter("rust-de-llama");
    let processed_tokens_counter = meter
        .u64_counter("processed_tokens_total")
        .with_description("Total number of tokens processed")
        .init();

    let model_manager = model_manager::ModelManager::new(
        args.model_directory.clone(),
        args.n_parallel,
        args.n_ctx,
        args.n_batch,
        args.n_ubatch,
    );

    let llama_backend = std::sync::Arc::new(model_manager);

    if let Some(model_name) = &args.preload_model {
        tracing::info!("Preloading model: {}", model_name);

        match llama_backend.get_or_load_model(model_name).await {
            Ok(_) => {
                tracing::info!("Successfully preloaded model: {}", model_name);
            }
            Err(e) => {
                tracing::warn!("Failed to preload model '{}': {}", model_name, e);
                tracing::warn!("The server will continue, models will be loaded on demand");
            }
        }
    }

    let router = axum::Router::new()
        .route("/healthz", axum::routing::get(handler::healthz))
        .route(
            "/v1/chat/completions",
            axum::routing::post(handler::chat_completions),
        )
        .with_state(AppState {
            llama_backend,
            processed_tokens_counter,
        });

    let monitor_router = axum::Router::new()
        .route(
            "/debug/pprof/profile",
            axum::routing::get(handler::debug::pprof::profile),
        )
        .route("/metrics", axum::routing::get(handler::metrics))
        .layer(axum::middleware::from_fn(middleware::propagator))
        .with_state(MonitorState { exporter });

    let (tx, mut rx1) = tokio::sync::broadcast::channel(1);
    let mut rx2 = rx1.resubscribe();

    let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    tokio::spawn(async move {
        tokio::select! {
            _ = sigterm.recv() => {
                tokio::time::sleep(std::time::Duration::from_secs(args.lameduck)).await;
                let _ = tx.send(());
            }
        }
    });
    let (server, monitor_server) = tokio::join!(
        axum::Server::bind(&args.address.parse::<std::net::SocketAddr>()?)
            .http1_keepalive(args.http_keepalive)
            .serve(router.into_make_service())
            .with_graceful_shutdown(async {
                let _ = rx1.recv().await;
            }),
        axum::Server::bind(&args.monitor_address.parse::<std::net::SocketAddr>()?)
            .http1_keepalive(args.http_keepalive)
            .serve(monitor_router.into_make_service())
            .with_graceful_shutdown(async {
                let _ = rx2.recv().await;
            }),
    );

    server?;
    monitor_server?;

    Ok(())
}
