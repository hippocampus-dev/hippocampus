use std::io::Write;

use hippocampus_core::storage::DocumentStorage;

#[derive(clap::Parser, Debug)]
pub struct Args {
    query: Option<String>,
    #[clap(short, long)]
    interactive: bool,
}

#[cfg_attr(feature = "tracing", tracing::instrument)]
pub async fn run(
    args: Args,
    configuration: &hippocampus_configuration::Configuration,
    schema: hippocampus_core::Schema,
) -> crate::Return {
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
    if args.interactive {
        if let Some(result) = crate::ui::run(searcher).await? {
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
        let (_, query) = hippocampusql::parse(&args.query.unwrap_or_default())?;
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
    Ok(())
}
