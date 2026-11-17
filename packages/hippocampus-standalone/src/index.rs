use futures::TryStreamExt;

#[derive(clap::Parser, Debug)]
pub struct Args {
    #[clap(name = "FILE")]
    files: Vec<std::path::PathBuf>,
    #[clap(short, long, default_value_t = 4)]
    concurrency: usize,
    #[clap(short, long, default_value_t = 10)]
    batch_size: usize,
}

#[cfg_attr(feature = "tracing", tracing::instrument)]
pub async fn run(
    args: Args,
    configuration: &hippocampus_configuration::Configuration,
    schema: hippocampus_core::Schema,
) -> crate::Return {
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

    let indexer: Box<dyn hippocampus_core::indexer::Indexer> = match &configuration.document_storage
    {
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
    let batches: Vec<Vec<std::path::PathBuf>> = args
        .files
        .chunks(args.batch_size)
        .map(|chunk| chunk.to_vec())
        .collect();

    futures::stream::iter(batches.into_iter().map(Ok))
        .try_for_each_concurrent(args.concurrency, |chunk| {
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
                        let content = std::fs::read_to_string(file).map_err(error::Error::from)?;
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
    Ok(())
}
