use hippocampus_core::indexer::Indexer;
use hippocampus_core::searcher::Searcher;
use hippocampus_core::storage::DocumentStorage;

#[tokio::main]
async fn main() -> Result<(), error::Error> {
    println!("Loading WASM tokenizer plugin from /tmp/regex_tokenizer.wasm");

    let tokenizer =
        hippocampus_core::tokenizer::wasm::WasmTokenizer::from_file("/tmp/regex_tokenizer.wasm")?;

    let documents_directory = tempfile::tempdir()?.into_path();
    let tokens_directory = tempfile::tempdir()?.into_path();

    let mut schema = hippocampus_core::Schema::new();
    schema.add_field(hippocampus_core::Field {
        name: "title".to_string(),
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

    let document_storage = hippocampus_core::storage::file::File::new(
        documents_directory.clone(),
        std::collections::hash_map::DefaultHasher::new(),
    );
    let token_storage = hippocampus_core::storage::file::File::new(
        tokens_directory.clone(),
        std::collections::hash_map::DefaultHasher::new(),
    );

    let indexer = hippocampus_core::indexer::DocumentIndexer::new(
        document_storage,
        token_storage,
        tokenizer.clone(),
        schema.clone(),
    );

    println!("Indexing documents...");
    let mut field_values = std::collections::BTreeMap::new();
    field_values.insert(
        "title".to_string(),
        hippocampus_core::Value::String("Sample Document".to_string()),
    );
    field_values.insert(
        "content".to_string(),
        hippocampus_core::Value::String("The quick brown fox jumps over the lazy dog!".to_string()),
    );
    indexer
        .index(vec![hippocampus_core::Document(field_values)])
        .await?;

    println!("Searching for 'quick'...");
    let document_storage = hippocampus_core::storage::file::File::new(
        documents_directory.clone(),
        std::collections::hash_map::DefaultHasher::new(),
    );
    let token_storage = hippocampus_core::storage::file::File::new(
        tokens_directory.clone(),
        std::collections::hash_map::DefaultHasher::new(),
    );
    let indexed_count = document_storage.count().await?;
    let scorer = hippocampus_core::scorer::tf_idf::TfIdf::new(indexed_count);
    let searcher = hippocampus_core::searcher::DocumentSearcher::new(
        document_storage,
        token_storage,
        tokenizer,
        scorer,
        schema,
    );

    let (_, query) = hippocampusql::parse("quick")?;
    let results = searcher
        .search(&query, hippocampus_core::searcher::SearchOption::default())
        .await?;

    println!("Found {} results:", results.len());
    for (rank, result) in results.iter().enumerate() {
        println!("  [{}] Score: {:.4}", rank + 1, result.score);
        println!("      Title: {:?}", result.document.get("title"));
        println!("      Content: {:?}", result.document.get("content"));
    }

    assert_eq!(results.len(), 1, "Should find exactly one document");
    println!("\nTest passed! WASM tokenizer plugin works correctly.");

    Ok(())
}
