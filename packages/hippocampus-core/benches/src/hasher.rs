#[cfg(test)]
mod tests {
    extern crate test;

    use hippocampus_core::storage::DocumentStorage;

    #[bench]
    fn bench_siphash_save(
        b: &mut test::Bencher,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let documents = tempfile::tempdir()?.into_path();

        let document_storage = hippocampus_core::storage::file::File::new(
            documents,
            std::collections::hash_map::DefaultHasher::new(),
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

        let rt = tokio::runtime::Runtime::new()?;
        b.iter(|| {
            rt.block_on(document_storage.save(&hippocampus_core::Document(field_values.clone())))
                .unwrap();
        });
        Ok(())
    }

    #[bench]
    fn bench_fxhash_save(
        b: &mut test::Bencher,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let documents = tempfile::tempdir()?.into_path();

        let document_storage =
            hippocampus_core::storage::file::File::new(documents, rustc_hash::FxHasher::default());

        let mut field_values = std::collections::BTreeMap::new();
        field_values.insert(
            "file".to_string(),
            hippocampus_core::Value::String("sample.txt".to_string()),
        );
        field_values.insert(
            "content".to_string(),
            hippocampus_core::Value::String("this is sample".repeat(100)),
        );

        let rt = tokio::runtime::Runtime::new()?;
        b.iter(|| {
            rt.block_on(document_storage.save(&hippocampus_core::Document(field_values.clone())))
                .unwrap();
        });
        Ok(())
    }

    #[bench]
    fn bench_ahash_save(
        b: &mut test::Bencher,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let documents = tempfile::tempdir()?.into_path();

        let document_storage =
            hippocampus_core::storage::file::File::new(documents, ahash::AHasher::default());

        let mut field_values = std::collections::BTreeMap::new();
        field_values.insert(
            "file".to_string(),
            hippocampus_core::Value::String("sample.txt".to_string()),
        );
        field_values.insert(
            "content".to_string(),
            hippocampus_core::Value::String("this is sample".repeat(100)),
        );

        let rt = tokio::runtime::Runtime::new()?;
        b.iter(|| {
            rt.block_on(document_storage.save(&hippocampus_core::Document(field_values.clone())))
                .unwrap();
        });
        Ok(())
    }

    #[bench]
    fn bench_siphash_save_big(
        b: &mut test::Bencher,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let documents = tempfile::tempdir()?.into_path();

        let document_storage = hippocampus_core::storage::file::File::new(
            documents,
            std::collections::hash_map::DefaultHasher::new(),
        );

        let mut field_values = std::collections::BTreeMap::new();
        field_values.insert(
            "file".to_string(),
            hippocampus_core::Value::String("sample.txt".to_string()),
        );
        field_values.insert(
            "content".to_string(),
            hippocampus_core::Value::String("this is sample".repeat(100_000)),
        );

        let rt = tokio::runtime::Runtime::new()?;
        b.iter(|| {
            rt.block_on(document_storage.save(&hippocampus_core::Document(field_values.clone())))
                .unwrap();
        });
        Ok(())
    }

    #[bench]
    fn bench_fxhash_save_big(
        b: &mut test::Bencher,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let documents = tempfile::tempdir()?.into_path();

        let document_storage =
            hippocampus_core::storage::file::File::new(documents, rustc_hash::FxHasher::default());

        let mut field_values = std::collections::BTreeMap::new();
        field_values.insert(
            "file".to_string(),
            hippocampus_core::Value::String("sample.txt".to_string()),
        );
        field_values.insert(
            "content".to_string(),
            hippocampus_core::Value::String("this is sample".repeat(100_000)),
        );

        let rt = tokio::runtime::Runtime::new()?;
        b.iter(|| {
            rt.block_on(document_storage.save(&hippocampus_core::Document(field_values.clone())))
                .unwrap();
        });
        Ok(())
    }

    #[bench]
    fn bench_ahash_save_big(
        b: &mut test::Bencher,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let documents = tempfile::tempdir()?.into_path();

        let document_storage =
            hippocampus_core::storage::file::File::new(documents, ahash::AHasher::default());

        let mut field_values = std::collections::BTreeMap::new();
        field_values.insert(
            "file".to_string(),
            hippocampus_core::Value::String("sample.txt".to_string()),
        );
        field_values.insert(
            "content".to_string(),
            hippocampus_core::Value::String("this is sample".repeat(100_000)),
        );

        let rt = tokio::runtime::Runtime::new()?;
        b.iter(|| {
            rt.block_on(document_storage.save(&hippocampus_core::Document(field_values.clone())))
                .unwrap();
        });
        Ok(())
    }
}
