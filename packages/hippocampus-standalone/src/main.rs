use std::io::Write;

use clap::Parser;
use futures::{StreamExt, TryFutureExt};
use opentelemetry_otlp::WithExportConfig;
use tracing_subscriber::{Layer, layer::SubscriberExt, util::SubscriberInitExt};

#[cfg(debug_assertions)]
use elapsed::prelude::*;
use hippocampus_core::storage::DocumentStorage;

use crate::config::{DocumentStorageKind, TokenStorageKind};

mod config;
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
struct Args {
    #[clap(subcommand)]
    sub_command: SubCommand,

    #[clap(short)]
    config_file_path: Option<std::path::PathBuf>,

    #[clap(long = "key-file")]
    key_file: Option<std::path::PathBuf>,
}

#[derive(clap::Parser, Debug)]
enum SubCommand {
    Index(Index),
    Search(Search),
}

#[derive(clap::Parser, Debug)]
struct Index {
    #[clap(name = "FILE")]
    files: Vec<std::path::PathBuf>,
    #[clap(short, long)]
    concurrency: Option<u64>,
}

#[derive(clap::Parser, Debug)]
struct Search {
    #[clap(name = "QUERY")]
    query: Option<String>,
    #[clap(short, long)]
    interactive: bool,
}

#[tokio::main]
async fn main() -> Return {
    #[cfg(debug_assertions)]
    dotenv::dotenv()?;

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
        let fmt = tracing_subscriber::fmt::layer()
            .with_filter(tracing_subscriber::EnvFilter::from_default_env());
        tracing_subscriber::registry()
            .with(fmt)
            .with(telemetry)
            .init();
    }

    let args: Args = Args::parse();
    process(args).await?;

    if cfg!(feature = "tracing") {
        opentelemetry::global::shutdown_tracer_provider();
    }

    Ok(())
}

#[cfg_attr(feature = "tracing", tracing::instrument)]
#[cfg_attr(debug_assertions, elapsed)]
async fn process(args: Args) -> Return {
    let config_body = std::fs::read(args.config_file_path.unwrap())?;
    let config: config::HippocampusConfig = toml::from_slice(&config_body)?;

    let mut schema = hippocampus_core::Schema::new();
    schema.add_field(hippocampus_core::Field {
        name: "file".to_string(),
        field_type: hippocampus_core::FieldType::String(hippocampus_core::StringOption {
            indexeing: false,
        }),
    });
    schema.add_field(hippocampus_core::Field {
        name: "content".to_string(),
        field_type: hippocampus_core::FieldType::String(hippocampus_core::StringOption {
            indexeing: true,
        }),
    });

    let tokenizer = hippocampus_core::tokenizer::lindera::Lindera::new()?;
    match args.sub_command {
        SubCommand::Index(index) => {
            let indexer = match (config.document_storage.kind, config.token_storage.kind) {
                (DocumentStorageKind::File, TokenStorageKind::File) => {
                    let document_storage = hippocampus_core::storage::file::File::new(
                        config.document_storage.file.unwrap().path.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );
                    let token_storage = hippocampus_core::storage::file::File::new(
                        config.token_storage.file.unwrap().path.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );
                    Box::new(hippocampus_core::indexer::DocumentIndexer::new(
                        document_storage,
                        token_storage,
                        tokenizer,
                        schema,
                    )) as Box<dyn hippocampus_core::indexer::Indexer>
                }
                (DocumentStorageKind::File, TokenStorageKind::GCS) => {
                    let document_storage = hippocampus_core::storage::file::File::new(
                        config.document_storage.file.unwrap().path.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );

                    let file = std::fs::File::open(args.key_file.unwrap())?;
                    let reader = std::io::BufReader::new(file);
                    let service_account_key: gcs::ServiceAccountKey =
                        serde_json::from_reader(reader)?;
                    let mut builder = gcs::Client::builder();
                    builder.set_connect_timeout(std::time::Duration::from_millis(100));
                    // When reaching MAX_CONCURRENT_STREAMS, hyper::client::Client does not create a new connection.
                    //builder.pool_max_idle_per_host(0);
                    let client = builder.build(service_account_key).unwrap();
                    let gcs_config = config.token_storage.gcs.unwrap();
                    let token_storage = hippocampus_core::storage::gcs::GCS::new(
                        client.clone(),
                        gcs_config.bucket.unwrap(),
                        gcs_config.prefix.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );
                    Box::new(hippocampus_core::indexer::DocumentIndexer::new(
                        document_storage,
                        token_storage,
                        tokenizer,
                        schema,
                    )) as Box<dyn hippocampus_core::indexer::Indexer>
                }
            };
            futures::stream::iter(index.files)
                .map(|file| {
                    let f = {
                        let mut field_values = std::collections::BTreeMap::new();
                        field_values.insert(
                            "file".to_string(),
                            hippocampus_core::Value::String(file.display().to_string()),
                        );
                        let content = std::fs::read_to_string(&file)
                            .map_err(error::Error::from)
                            .unwrap();
                        field_values.insert(
                            "content".to_string(),
                            hippocampus_core::Value::String(content),
                        );
                        futures::future::ok(field_values)
                    };
                    f.and_then(|content| indexer.index(hippocampus_core::Document(content)))
                })
                .for_each_concurrent(index.concurrency.unwrap_or(10) as usize, |job| async {
                    if let Err(e) = job.await {
                        eprintln!("Error occurred while indexing a file: {e}")
                    }
                })
                .await;
        }
        SubCommand::Search(search) => {
            let searcher = match (config.document_storage.kind, config.token_storage.kind) {
                (DocumentStorageKind::File, TokenStorageKind::File) => {
                    let document_storage = hippocampus_core::storage::file::File::new(
                        config.document_storage.file.unwrap().path.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );
                    let token_storage = hippocampus_core::storage::file::File::new(
                        config.token_storage.file.unwrap().path.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );
                    let indexed_count = document_storage.count().await?;
                    let scorer = hippocampus_core::scorer::tf_idf::TfIdf::new(indexed_count);
                    Box::new(hippocampus_core::searcher::DocumentSearcher::new(
                        document_storage,
                        token_storage,
                        tokenizer,
                        scorer,
                        schema,
                    ))
                        as Box<dyn Send + Sync + hippocampus_core::searcher::Searcher>
                }
                (DocumentStorageKind::File, TokenStorageKind::GCS) => {
                    let document_storage = hippocampus_core::storage::file::File::new(
                        config.document_storage.file.unwrap().path.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );

                    let file = std::fs::File::open(args.key_file.unwrap())?;
                    let reader = std::io::BufReader::new(file);
                    let service_account_key: gcs::ServiceAccountKey =
                        serde_json::from_reader(reader)?;
                    let mut builder = gcs::Client::builder();
                    builder.set_connect_timeout(std::time::Duration::from_millis(100));
                    // When reaching MAX_CONCURRENT_STREAMS, hyper::client::Client does not create a new connection.
                    //builder.pool_max_idle_per_host(0);
                    let client = builder.build(service_account_key).unwrap();
                    let gcs_config = config.token_storage.gcs.unwrap();
                    let token_storage = hippocampus_core::storage::gcs::GCS::new(
                        client,
                        gcs_config.bucket.unwrap(),
                        gcs_config.prefix.unwrap(),
                        rustc_hash::FxHasher::default(),
                    );
                    let indexed_count = document_storage.count().await?;
                    let scorer = hippocampus_core::scorer::tf_idf::TfIdf::new(indexed_count);
                    Box::new(hippocampus_core::searcher::DocumentSearcher::new(
                        document_storage,
                        token_storage,
                        tokenizer,
                        scorer,
                        schema,
                    ))
                        as Box<dyn Send + Sync + hippocampus_core::searcher::Searcher>
                }
            };
            if search.interactive {
                if let Some(result) = ui::run(searcher).await? {
                    println!(
                        "Document {}(score: {})\n{}",
                        result.document.get("file").unwrap(),
                        result.score,
                        result
                            .fragments
                            .into_iter()
                            .collect::<Vec<String>>()
                            .join("\n")
                    )
                }
            } else {
                let (_, query) = hippocampusql::parse(&search.query.unwrap_or_default())?;
                let results = searcher
                    .search(&query, hippocampus_core::searcher::SearchOption::default())
                    .await?;
                let stdout = std::io::stdout();
                let mut out = std::io::BufWriter::new(stdout.lock());
                writeln!(out, "Found {} documents", results.len())?;
                for result in results {
                    writeln!(
                        out,
                        "Document {}(score: {})\n{}",
                        result.document.get("file").unwrap(),
                        result.score,
                        result
                            .fragments
                            .into_iter()
                            .collect::<Vec<String>>()
                            .join("\n")
                    )?
                }
                out.flush()?;
            }
        }
    }

    Ok(())
}
