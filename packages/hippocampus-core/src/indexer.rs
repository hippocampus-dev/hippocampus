use futures::StreamExt;

#[cfg(debug_assertions)]
use elapsed::prelude::*;

#[async_trait::async_trait]
pub trait Indexer {
    async fn index(&self, document: crate::Document) -> Result<(), error::Error>;
}

#[derive(Clone, Debug)]
pub struct DocumentIndexer<
    DS: Send + Sync + crate::storage::DocumentStorage,
    TS: Send + Sync + crate::storage::TokenStorage,
    T: Clone + Send + Sync + crate::tokenizer::Tokenizer,
> {
    document_storage: DS,
    token_storage: TS,
    tokenizer: T,
    schema: crate::Schema,
}

impl<
        DS: Send + Sync + crate::storage::DocumentStorage,
        TS: Send + Sync + crate::storage::TokenStorage,
        T: Clone + Send + Sync + crate::tokenizer::Tokenizer,
    > DocumentIndexer<DS, TS, T>
{
    pub fn new(
        document_storage: DS,
        token_storage: TS,
        tokenizer: T,
        schema: crate::Schema,
    ) -> DocumentIndexer<DS, TS, T> {
        Self {
            document_storage,
            token_storage,
            tokenizer,
            schema,
        }
    }
}

#[async_trait::async_trait]
impl<
        DS: Send + Sync + crate::storage::DocumentStorage,
        TS: Send + Sync + crate::storage::TokenStorage,
        T: Clone + Send + Sync + crate::tokenizer::Tokenizer + 'static,
    > Indexer for DocumentIndexer<DS, TS, T>
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn index(&self, document: crate::Document) -> Result<(), error::Error> {
        let document_uuid = self.document_storage.save(&document).await?;
        let mut t = std::collections::HashMap::<String, crate::Posting>::new();
        for (field, value) in document.iter() {
            if !self.schema.get_field(field).unwrap().is_indexed() {
                continue;
            }
            match value {
                crate::Value::String(body) => {
                    let mut tokenizer = self.tokenizer.clone();
                    let body = body.clone();
                    let tokens =
                        tokio::task::spawn_blocking(move || tokenizer.tokenize(body)).await??;
                    for (i, token) in tokens.iter().enumerate() {
                        if let Some(posting) = t.get_mut(token) {
                            posting.positions.insert(i as u64);
                        } else {
                            t.insert(
                                token.clone(),
                                crate::Posting::new(
                                    document_uuid,
                                    vec![i as u64].into_iter().collect(),
                                ),
                            );
                        }
                    }
                }
            }
        }

        futures::stream::iter(t)
            .map(|(token, posting)| {
                let index = crate::InvertedIndex::new(token, crate::PostingsList(vec![posting]));
                self.token_storage.save_postings_list(index)
            })
            .for_each_concurrent(100, |job| async {
                if let Err(e) = job.await {
                    eprintln!("error ocurrered when indexing a token: {}", e)
                }
            })
            .await;

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use std::iter::FromIterator;

    use super::*;

    #[tokio::test]
    async fn index() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mut mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_save_postings_list()
            .with(mockall::predicate::eq(crate::InvertedIndex {
                token: "dummy".to_string(),
                postings_list: crate::PostingsList(vec![crate::Posting::new(
                    1,
                    std::collections::BTreeSet::from_iter(vec![0]),
                )]),
            }))
            .times(1)
            .returning(|_| Box::pin(async { Ok(()) }));
        let mut bm = std::collections::BTreeMap::new();
        bm.insert(
            "content".to_string(),
            crate::Value::String("dummy".to_string()),
        );
        mock_document_storage
            .expect_save()
            .with(mockall::predicate::eq(crate::Document(bm.clone())))
            .times(1)
            .returning(|_| Box::pin(async { Ok(1) }));
        let mut schema = crate::Schema::new();
        schema.add_field(crate::Field {
            name: "content".to_string(),
            field_type: crate::FieldType::String(crate::StringOption { indexeing: true }),
        });
        let indexer = DocumentIndexer::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            schema,
        );

        assert!(indexer.index(crate::Document(bm)).await.is_ok());
        Ok(())
    }

    #[tokio::test]
    async fn index_with_multiple_token() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mut mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_save_postings_list()
            .with(mockall::predicate::eq(crate::InvertedIndex {
                token: "dummy".to_string(),
                postings_list: crate::PostingsList(vec![crate::Posting::new(
                    1,
                    std::collections::BTreeSet::from_iter(vec![0, 2]),
                )]),
            }))
            .times(1)
            .returning(|_| Box::pin(async { Ok(()) }));
        mock_token_storage
            .expect_save_postings_list()
            .with(mockall::predicate::eq(crate::InvertedIndex {
                token: " ".to_string(),
                postings_list: crate::PostingsList(vec![crate::Posting::new(
                    1,
                    std::collections::BTreeSet::from_iter(vec![1]),
                )]),
            }))
            .times(1)
            .returning(|_| Box::pin(async { Ok(()) }));
        let mut bm = std::collections::BTreeMap::new();
        bm.insert(
            "content".to_string(),
            crate::Value::String("dummy dummy".to_string()),
        );
        mock_document_storage
            .expect_save()
            .with(mockall::predicate::eq(crate::Document(bm.clone())))
            .times(1)
            .returning(|_| Box::pin(async { Ok(1) }));
        let mut schema = crate::Schema::new();
        schema.add_field(crate::Field {
            name: "content".to_string(),
            field_type: crate::FieldType::String(crate::StringOption { indexeing: true }),
        });
        let indexer = DocumentIndexer::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            schema,
        );

        assert!(indexer.index(crate::Document(bm)).await.is_ok());
        Ok(())
    }
}
