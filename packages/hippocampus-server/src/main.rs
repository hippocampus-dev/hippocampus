use clap::Parser;
use opentelemetry::metrics::MeterProvider;
use opentelemetry_otlp::WithExportConfig;
use tracing_subscriber::{Layer, layer::SubscriberExt, util::SubscriberInitExt};

mod handler;
mod middleware;
mod service;

#[cfg(not(feature = "jemalloc"))]
#[global_allocator]
static GLOBAL: std::alloc::System = std::alloc::System;

#[cfg(feature = "jemalloc")]
#[global_allocator]
static GLOBAL: tikv_jemallocator::Jemalloc = tikv_jemallocator::Jemalloc;

type Return = Result<(), error::Error>;

#[derive(clap::Parser, Debug)]
struct Args {
    #[clap(short, long)]
    config_file_path: Option<std::path::PathBuf>,

    #[clap(long = "key-file")]
    key_file: Option<std::path::PathBuf>,

    #[clap(long, default_value = "127.0.0.1:8080")]
    address: String,
    #[clap(long = "monitor-address", default_value = "127.0.0.1:8081")]
    monitor_address: String,

    /// A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service
    #[clap(long, default_value_t = 1)]
    lameduck: u64,

    #[clap(long = "http-keepalive", default_value_t = true)]
    http_keepalive: bool,
}

fn main() -> Return {
    #[cfg(debug_assertions)]
    dotenv::dotenv()?;

    let args: Args = Args::parse();

    tokio::runtime::Builder::new_multi_thread()
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
pub struct AppState {
    http_requests_total: opentelemetry::metrics::Counter<u64>,
}

#[derive(Clone, Debug)]
pub struct MonitorState {
    exporter: opentelemetry_prometheus::PrometheusExporter,
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

    let meter = exporter.meter_provider()?.meter("hippocampus-server");
    let http_requests_total = meter.u64_counter("http_requests_total").init();

    let monitor_router = axum::Router::new()
        .route(
            "/debug/pprof/profile",
            axum::routing::get(handler::debug::pprof::profile),
        )
        .route("/metrics", axum::routing::get(handler::metrics))
        .layer(middleware::TracingLayer)
        .with_state(MonitorState { exporter });

    let greeter = service::MyGreeter::new(http_requests_total);
    let (_, health_server) = tonic_health::server::health_reporter();

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
        tonic::transport::Server::builder()
            .layer(middleware::TracingLayer)
            .add_service(service::hello_world::greeter_server::GreeterServer::new(
                greeter
            ))
            .add_service(health_server)
            .serve_with_shutdown(args.address.parse::<std::net::SocketAddr>()?, async {
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
