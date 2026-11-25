use clap::Parser;
use opentelemetry::metrics::MeterProvider;
use opentelemetry::trace::TracerProvider;
use opentelemetry_otlp::WithTonicConfig;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

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
pub struct Args {
    #[clap(short, long)]
    config_file_path: Option<std::path::PathBuf>,

    #[clap(long, default_value = "127.0.0.1:8080")]
    address: String,
    #[clap(long, default_value = "127.0.0.1:8081")]
    monitor_address: String,

    /// A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service
    #[clap(long, default_value_t = 1)]
    lameduck: u64,
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
                opentelemetry_sdk::propagation::TraceContextPropagator::new(),
            );

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

            let result = process(args).await;

            if let Some(tracer_provider) = tracer_provider {
                // opentelemetry_sdk already logs a failed final export as BatchSpanProcessor.ExportError
                let _ = tracer_provider.shutdown();
            }

            result
        })?;

    Ok(())
}

#[derive(Clone, Debug)]
pub struct AppState {
    http_requests_total: opentelemetry::metrics::Counter<u64>,
}

#[derive(Clone, Debug)]
pub struct MonitorState {
    registry: prometheus::Registry,
}

async fn process(args: Args) -> Return {
    let registry = prometheus::Registry::new();
    let exporter = opentelemetry_prometheus::exporter()
        .with_registry(registry.clone())
        .without_counter_suffixes()
        .without_scope_info()
        .without_target_info()
        .build()?;
    let meter_provider = opentelemetry_sdk::metrics::SdkMeterProvider::builder()
        .with_reader(exporter)
        .build();

    let meter = meter_provider.meter("hippocampus-server");
    let http_requests_total = meter.u64_counter("http_requests_total").build();

    let monitor_router = axum::Router::new()
        .route(
            "/debug/pprof/profile",
            axum::routing::get(handler::debug::pprof::profile),
        )
        .route("/metrics", axum::routing::get(handler::metrics))
        .route("/openapi.json", axum::routing::get(handler::openapi::spec))
        .layer(middleware::TracingLayer)
        .with_state(MonitorState { registry });

    let greeter = service::MyGreeter::new(http_requests_total.clone());
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

    let mut server_builder = tonic::transport::Server::builder()
        .layer(middleware::TracingLayer)
        .add_service(service::hello_world::greeter_server::GreeterServer::new(
            greeter,
        ))
        .add_service(health_server);

    macro_rules! register_hippocampus_service {
        ($server_builder:expr, $document_storage:expr, $token_storage:expr, $tokenizer_config:expr, $schema:expr, $http_requests_total:expr) => {
            match $tokenizer_config {
                hippocampus_configuration::TokenizerConfiguration::Lindera => {
                    let tokenizer = hippocampus_core::tokenizer::lindera::Lindera::new()?;
                    let hippocampus_service = service::HippocampusService::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        $schema,
                        $http_requests_total,
                    );
                    $server_builder = $server_builder.add_service(
                        service::hippocampus::hippocampus_server::HippocampusServer::new(
                            hippocampus_service,
                        ),
                    );
                }
                hippocampus_configuration::TokenizerConfiguration::Whitespace => {
                    let tokenizer = hippocampus_core::tokenizer::whitespace::Whitespace {};
                    let hippocampus_service = service::HippocampusService::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        $schema,
                        $http_requests_total,
                    );
                    $server_builder = $server_builder.add_service(
                        service::hippocampus::hippocampus_server::HippocampusServer::new(
                            hippocampus_service,
                        ),
                    );
                }
                #[cfg(feature = "wasm")]
                hippocampus_configuration::TokenizerConfiguration::Wasm { path } => {
                    let tokenizer =
                        hippocampus_core::tokenizer::wasm::WasmTokenizer::from_file(path)?;
                    let hippocampus_service = service::HippocampusService::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        $schema,
                        $http_requests_total,
                    );
                    $server_builder = $server_builder.add_service(
                        service::hippocampus::hippocampus_server::HippocampusServer::new(
                            hippocampus_service,
                        ),
                    );
                }
            }
        };
    }

    macro_rules! with_token_storage {
        ($server_builder:expr, $document_storage:expr, $token_storage_config:expr, $tokenizer_config:expr, $schema:expr, $http_requests_total:expr) => {
            match $token_storage_config {
                hippocampus_configuration::TokenStorageConfiguration::File { path } => {
                    std::fs::create_dir_all(path)?;
                    let token_storage = hippocampus_core::storage::file::File::new(
                        path.clone(),
                        rustc_hash::FxHasher::default(),
                    );
                    register_hippocampus_service!(
                        $server_builder,
                        $document_storage,
                        token_storage,
                        $tokenizer_config,
                        $schema,
                        $http_requests_total
                    );
                }
                #[cfg(feature = "sqlite")]
                hippocampus_configuration::TokenStorageConfiguration::SQLite { path } => {
                    let token_storage = hippocampus_core::storage::sqlite::SQLite::new(
                        Some(path.clone()),
                        rustc_hash::FxHasher::default(),
                    )?;
                    register_hippocampus_service!(
                        $server_builder,
                        $document_storage,
                        token_storage,
                        $tokenizer_config,
                        $schema,
                        $http_requests_total
                    );
                }
                #[cfg(feature = "cassandra")]
                hippocampus_configuration::TokenStorageConfiguration::Cassandra { address } => {
                    let token_storage = hippocampus_core::storage::cassandra::Cassandra::new(
                        address,
                        rustc_hash::FxHasher::default(),
                    )
                    .await?;
                    register_hippocampus_service!(
                        $server_builder,
                        $document_storage,
                        token_storage,
                        $tokenizer_config,
                        $schema,
                        $http_requests_total
                    );
                }
                hippocampus_configuration::TokenStorageConfiguration::GCS {
                    bucket,
                    prefix,
                    service_account_key_path,
                } => {
                    let service_account_key_content =
                        std::fs::read_to_string(service_account_key_path)?;
                    let service_account_key: gcs::ServiceAccountKey =
                        serde_json::from_str(&service_account_key_content)?;
                    let mut builder = gcs::Client::builder();
                    builder.set_connect_timeout(std::time::Duration::from_millis(100));
                    let gcs_client = builder.build(service_account_key)?;
                    let token_storage = hippocampus_core::storage::gcs::GCS::new(
                        gcs_client,
                        bucket.clone(),
                        prefix.clone(),
                        rustc_hash::FxHasher::default(),
                    );
                    register_hippocampus_service!(
                        $server_builder,
                        $document_storage,
                        token_storage,
                        $tokenizer_config,
                        $schema,
                        $http_requests_total
                    );
                }
            }
        };
    }

    if let Some(configuration_file_path) = &args.config_file_path {
        let configuration =
            hippocampus_configuration::Configuration::from_file(configuration_file_path)?;
        let mut schema = hippocampus_core::Schema::new();
        for field in &configuration.schema.fields {
            schema.add_field(hippocampus_core::Field {
                name: field.name.clone(),
                field_type: match field.field_type {
                    hippocampus_configuration::FieldType::String => {
                        hippocampus_core::FieldType::String(hippocampus_core::StringOption {
                            indexeing: field.indexed,
                        })
                    }
                },
            });
        }

        match &configuration.document_storage {
            hippocampus_configuration::DocumentStorageConfiguration::File { path } => {
                std::fs::create_dir_all(path)?;
                let document_storage = hippocampus_core::storage::file::File::new(
                    path.clone(),
                    rustc_hash::FxHasher::default(),
                );
                with_token_storage!(
                    server_builder,
                    document_storage,
                    &configuration.token_storage,
                    &configuration.tokenizer,
                    schema,
                    http_requests_total
                );
            }
            #[cfg(feature = "sqlite")]
            hippocampus_configuration::DocumentStorageConfiguration::SQLite { path } => {
                let document_storage = hippocampus_core::storage::sqlite::SQLite::new(
                    Some(path.clone()),
                    rustc_hash::FxHasher::default(),
                )?;
                with_token_storage!(
                    server_builder,
                    document_storage,
                    &configuration.token_storage,
                    &configuration.tokenizer,
                    schema,
                    http_requests_total
                );
            }
        }
    }

    let monitor_listener = tokio::net::TcpListener::bind(&args.monitor_address).await?;

    let (server, monitor_server) = tokio::join!(
        server_builder.serve_with_shutdown(
            args.address.parse::<std::net::SocketAddr>()?,
            async move {
                let _ = rx1.recv().await;
            }
        ),
        axum::serve(monitor_listener, monitor_router).with_graceful_shutdown(async move {
            let _ = rx2.recv().await;
        }),
    );

    server?;
    monitor_server?;

    Ok(())
}
