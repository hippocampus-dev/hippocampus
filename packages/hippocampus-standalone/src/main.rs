use std::io::Write;

use clap::Parser;
use futures::TryStreamExt;
use opentelemetry_otlp::WithExportConfig;
use tracing_subscriber::{Layer, layer::SubscriberExt, util::SubscriberInitExt};

#[cfg(debug_assertions)]
use elapsed::prelude::*;
use hippocampus_core::storage::DocumentStorage;

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
    #[clap(short, long, default_value = "4")]
    concurrency: usize,
    #[clap(short, long, default_value = "10")]
    batch_size: usize,
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
    let configuration =
        hippocampus_configuration::Configuration::from_file(&args.config_file_path.unwrap())?;

    let schema = schema_from_configuration(&configuration.schema);

    macro_rules! create_indexer {
        ($document_storage:expr, $token_storage:expr, $tokenizer_config:expr, $schema:expr) => {
            match $tokenizer_config {
                hippocampus_configuration::TokenizerConfiguration::Lindera => {
                    let tokenizer = hippocampus_core::tokenizer::lindera::Lindera::new()?;
                    Box::new(hippocampus_core::indexer::DocumentIndexer::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        $schema,
                    )) as Box<dyn hippocampus_core::indexer::Indexer>
                }
                hippocampus_configuration::TokenizerConfiguration::Whitespace => {
                    let tokenizer = hippocampus_core::tokenizer::whitespace::Whitespace {};
                    Box::new(hippocampus_core::indexer::DocumentIndexer::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        $schema,
                    )) as Box<dyn hippocampus_core::indexer::Indexer>
                }
                #[cfg(feature = "wasm")]
                hippocampus_configuration::TokenizerConfiguration::Wasm { path } => {
                    let tokenizer =
                        hippocampus_core::tokenizer::wasm::WasmTokenizer::from_file(path)?;
                    Box::new(hippocampus_core::indexer::DocumentIndexer::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        $schema,
                    )) as Box<dyn hippocampus_core::indexer::Indexer>
                }
            }
        };
    }

    macro_rules! create_searcher {
        ($document_storage:expr, $token_storage:expr, $tokenizer_config:expr, $schema:expr) => {{
            let indexed_count = $document_storage.count().await?;
            let scorer = hippocampus_core::scorer::tf_idf::TfIdf::new(indexed_count);
            match $tokenizer_config {
                hippocampus_configuration::TokenizerConfiguration::Lindera => {
                    let tokenizer = hippocampus_core::tokenizer::lindera::Lindera::new()?;
                    Box::new(hippocampus_core::searcher::DocumentSearcher::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        scorer,
                        $schema,
                    ))
                        as Box<dyn Send + Sync + hippocampus_core::searcher::Searcher>
                }
                hippocampus_configuration::TokenizerConfiguration::Whitespace => {
                    let tokenizer = hippocampus_core::tokenizer::whitespace::Whitespace {};
                    Box::new(hippocampus_core::searcher::DocumentSearcher::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        scorer,
                        $schema,
                    ))
                        as Box<dyn Send + Sync + hippocampus_core::searcher::Searcher>
                }
                #[cfg(feature = "wasm")]
                hippocampus_configuration::TokenizerConfiguration::Wasm { path } => {
                    let tokenizer =
                        hippocampus_core::tokenizer::wasm::WasmTokenizer::from_file(path)?;
                    Box::new(hippocampus_core::searcher::DocumentSearcher::new(
                        $document_storage,
                        $token_storage,
                        tokenizer,
                        scorer,
                        $schema,
                    ))
                        as Box<dyn Send + Sync + hippocampus_core::searcher::Searcher>
                }
            }
        }};
    }

    macro_rules! with_token_storage_index {
        ($document_storage:expr, $token_storage_config:expr, $tokenizer_config:expr, $schema:expr) => {
            match $token_storage_config {
                hippocampus_configuration::TokenStorageConfiguration::File { path } => {
                    std::fs::create_dir_all(path)?;
                    let token_storage = hippocampus_core::storage::file::File::new(
                        path.clone(),
                        rustc_hash::FxHasher::default(),
                    );
                    create_indexer!($document_storage, token_storage, $tokenizer_config, $schema)
                }
                #[cfg(feature = "sqlite")]
                hippocampus_configuration::TokenStorageConfiguration::SQLite { path } => {
                    let token_storage = hippocampus_core::storage::sqlite::SQLite::new(
                        Some(path.clone()),
                        rustc_hash::FxHasher::default(),
                    )?;
                    create_indexer!($document_storage, token_storage, $tokenizer_config, $schema)
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
                    let client = builder.build(service_account_key)?;
                    let token_storage = hippocampus_core::storage::gcs::GCS::new(
                        client,
                        bucket.clone(),
                        prefix.clone(),
                        rustc_hash::FxHasher::default(),
                    );
                    create_indexer!($document_storage, token_storage, $tokenizer_config, $schema)
                }
                #[cfg(feature = "cassandra")]
                hippocampus_configuration::TokenStorageConfiguration::Cassandra { address } => {
                    let token_storage = hippocampus_core::storage::cassandra::Cassandra::new(
                        address,
                        rustc_hash::FxHasher::default(),
                    )
                    .await?;
                    create_indexer!($document_storage, token_storage, $tokenizer_config, $schema)
                }
            }
        };
    }

    macro_rules! with_token_storage_search {
        ($document_storage:expr, $token_storage_config:expr, $tokenizer_config:expr, $schema:expr) => {
            match $token_storage_config {
                hippocampus_configuration::TokenStorageConfiguration::File { path } => {
                    std::fs::create_dir_all(path)?;
                    let token_storage = hippocampus_core::storage::file::File::new(
                        path.clone(),
                        rustc_hash::FxHasher::default(),
                    );
                    create_searcher!($document_storage, token_storage, $tokenizer_config, $schema)
                }
                #[cfg(feature = "sqlite")]
                hippocampus_configuration::TokenStorageConfiguration::SQLite { path } => {
                    let token_storage = hippocampus_core::storage::sqlite::SQLite::new(
                        Some(path.clone()),
                        rustc_hash::FxHasher::default(),
                    )?;
                    create_searcher!($document_storage, token_storage, $tokenizer_config, $schema)
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
                    let client = builder.build(service_account_key)?;
                    let token_storage = hippocampus_core::storage::gcs::GCS::new(
                        client,
                        bucket.clone(),
                        prefix.clone(),
                        rustc_hash::FxHasher::default(),
                    );
                    create_searcher!($document_storage, token_storage, $tokenizer_config, $schema)
                }
                #[cfg(feature = "cassandra")]
                hippocampus_configuration::TokenStorageConfiguration::Cassandra { address } => {
                    let token_storage = hippocampus_core::storage::cassandra::Cassandra::new(
                        address,
                        rustc_hash::FxHasher::default(),
                    )
                    .await?;
                    create_searcher!($document_storage, token_storage, $tokenizer_config, $schema)
                }
            }
        };
    }

    match args.sub_command {
        SubCommand::Index(index) => {
            let indexer: Box<dyn hippocampus_core::indexer::Indexer> =
                match &configuration.document_storage {
                    hippocampus_configuration::DocumentStorageConfiguration::File { path } => {
                        std::fs::create_dir_all(path)?;
                        let document_storage = hippocampus_core::storage::file::File::new(
                            path.clone(),
                            rustc_hash::FxHasher::default(),
                        );
                        with_token_storage_index!(
                            document_storage,
                            &configuration.token_storage,
                            &configuration.tokenizer,
                            schema
                        )
                    }
                    #[cfg(feature = "sqlite")]
                    hippocampus_configuration::DocumentStorageConfiguration::SQLite { path } => {
                        let document_storage = hippocampus_core::storage::sqlite::SQLite::new(
                            Some(path.clone()),
                            rustc_hash::FxHasher::default(),
                        )?;
                        with_token_storage_index!(
                            document_storage,
                            &configuration.token_storage,
                            &configuration.tokenizer,
                            schema
                        )
                    }
                };
            let indexer = std::sync::Arc::new(indexer);
            let batches: Vec<Vec<std::path::PathBuf>> = index
                .files
                .chunks(index.batch_size)
                .map(|chunk| chunk.to_vec())
                .collect();

            futures::stream::iter(batches.into_iter().map(Ok))
                .try_for_each_concurrent(index.concurrency, |chunk| {
                    let indexer = indexer.clone();
                    async move {
                        let documents: Result<Vec<_>, error::Error> = chunk
                            .iter()
                            .map(|file| {
                                let mut field_values = std::collections::BTreeMap::new();
                                field_values.insert(
                                    "file".to_string(),
                                    hippocampus_core::Value::String(file.display().to_string()),
                                );
                                let content =
                                    std::fs::read_to_string(file).map_err(error::Error::from)?;
                                field_values.insert(
                                    "content".to_string(),
                                    hippocampus_core::Value::String(content),
                                );
                                Ok(hippocampus_core::Document(field_values))
                            })
                            .collect();
                        indexer.index(documents?).await
                    }
                })
                .await?;
        }
        SubCommand::Search(search) => {
            let searcher: Box<dyn Send + Sync + hippocampus_core::searcher::Searcher> =
                match &configuration.document_storage {
                    hippocampus_configuration::DocumentStorageConfiguration::File { path } => {
                        std::fs::create_dir_all(path)?;
                        let document_storage = hippocampus_core::storage::file::File::new(
                            path.clone(),
                            rustc_hash::FxHasher::default(),
                        );
                        with_token_storage_search!(
                            document_storage,
                            &configuration.token_storage,
                            &configuration.tokenizer,
                            schema
                        )
                    }
                    #[cfg(feature = "sqlite")]
                    hippocampus_configuration::DocumentStorageConfiguration::SQLite { path } => {
                        let document_storage = hippocampus_core::storage::sqlite::SQLite::new(
                            Some(path.clone()),
                            rustc_hash::FxHasher::default(),
                        )?;
                        with_token_storage_search!(
                            document_storage,
                            &configuration.token_storage,
                            &configuration.tokenizer,
                            schema
                        )
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
