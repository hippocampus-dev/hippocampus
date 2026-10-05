use clap::Parser;
use opentelemetry_otlp::WithExportConfig;
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

fn schema_from_configuration(
    configuration: &hippocampus_configuration::SchemaConfiguration,
) -> hippocampus_core::Schema {
    let mut schema = hippocampus_core::Schema::new();
    for field in &configuration.fields {
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
    schema
}

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
            .install_simple()?;
        let telemetry = tracing_opentelemetry::layer().with_tracer(tracer);

        let formatter = tracing_subscriber::fmt::format().with_source_location(true);
        let fmt = tracing_subscriber::fmt::layer().event_format(formatter);

        tracing_subscriber::registry()
            .with(fmt)
            .with(telemetry)
            .with(tracing_subscriber::EnvFilter::from_default_env())
            .init();
    }

    process(args).await?;

    if cfg!(feature = "tracing") {
        opentelemetry::global::shutdown_tracer_provider();
    }

    Ok(())
}

#[cfg_attr(feature = "tracing", tracing::instrument)]
#[cfg_attr(debug_assertions, elapsed)]
async fn process(args: Args) -> Return {
    let configuration =
        hippocampus_configuration::Configuration::from_file(&args.config_file_path)?;

    let schema = schema_from_configuration(&configuration.schema);

    match args.sub_command {
        SubCommand::Index(args) => index::run(args, &configuration, schema).await,
        SubCommand::Search(args) => search::run(args, &configuration, schema).await,
    }
}
