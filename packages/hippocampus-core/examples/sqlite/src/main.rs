use hippocampus_core::indexer::Indexer;
use hippocampus_core::searcher::Searcher;
use hippocampus_core::storage::DocumentStorage;

#[tokio::main]
async fn main() -> Result<(), error::Error> {
    let p = tempfile::tempdir()?.into_path().join("sqlite.db");

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
    let document_storage = hippocampus_core::storage::sqlite::SQLite::new(
        Some(p.clone()),
        std::collections::hash_map::DefaultHasher::new(),
    )?;
    let token_storage = hippocampus_core::storage::sqlite::SQLite::new(
        Some(p.clone()),
        std::collections::hash_map::DefaultHasher::new(),
    )?;
    let indexer = hippocampus_core::indexer::DocumentIndexer::new(
        document_storage,
        token_storage,
        tokenizer,
        schema.clone(),
    );
    let mut field_values = std::collections::BTreeMap::new();
    field_values.insert(
        "file".to_string(),
        hippocampus_core::Value::String("sample.txt".to_string()),
    );
    field_values.insert(
        "content".to_string(),
        hippocampus_core::Value::String("this is sample".repeat(100)),
    );
    indexer
        .index(vec![hippocampus_core::Document(field_values)])
        .await?;

    let tokenizer = hippocampus_core::tokenizer::lindera::Lindera::new()?;
    let document_storage = hippocampus_core::storage::sqlite::SQLite::new(
        Some(p.clone()),
        std::collections::hash_map::DefaultHasher::new(),
    )?;
    let token_storage = hippocampus_core::storage::sqlite::SQLite::new(
        Some(p.clone()),
        std::collections::hash_map::DefaultHasher::new(),
    )?;
    let indexed_count = document_storage.count().await?;
    let scorer = hippocampus_core::scorer::tf_idf::TfIdf::new(indexed_count);
    let searcher = hippocampus_core::searcher::DocumentSearcher::new(
        document_storage,
        token_storage,
        tokenizer,
        scorer,
        schema,
    );
    let (_, query) = hippocampusql::parse("sample")?;
    let results = searcher
        .search(&query, hippocampus_core::searcher::SearchOption::default())
        .await?;
    assert_eq!(results.len(), 1);
    dbg!(&results);

    Ok(())
}
