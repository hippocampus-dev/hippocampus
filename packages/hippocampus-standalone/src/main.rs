use clap::Parser;
use opentelemetry::trace::TracerProvider;
use opentelemetry_otlp::WithTonicConfig;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[cfg(debug_assertions)]
use elapsed::prelude::*;

mod index;
mod search;
mod ui;

#[cfg(not(feature = "jemalloc"))]
#[global_allocator]
static GLOBAL: std::alloc::System = std::alloc::System;

#[cfg(feature = "jemalloc")]
#[global_allocator]
static GLOBAL: tikv_jemallocator::Jemalloc = tikv_jemallocator::Jemalloc;

type Return = Result<(), error::Error>;

/// Hippocampus Standalone is a client that runs on terminal for Hippocampus.
#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(subcommand)]
    sub_command: SubCommand,

    #[clap(short, long, default_value = "hippocampus.toml")]
    config_file_path: std::path::PathBuf,
}

#[derive(clap::Parser, Debug)]
enum SubCommand {
    /// Index the given files into the configured storages
    Index(index::Args),
    /// Search the indexed documents
    Search(search::Args),
}

#[tokio::main]
async fn main() -> Return {
    #[cfg(debug_assertions)]
    dotenv::dotenv()?;

    let args: Args = Args::parse();

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
}

#[cfg_attr(feature = "tracing", tracing::instrument)]
#[cfg_attr(debug_assertions, elapsed)]
async fn process(args: Args) -> Return {
    let configuration =
        hippocampus_configuration::Configuration::from_file(&args.config_file_path)?;

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

    match args.sub_command {
        SubCommand::Index(args) => index::run(args, &configuration, schema).await,
        SubCommand::Search(args) => search::run(args, &configuration, schema).await,
    }
}
