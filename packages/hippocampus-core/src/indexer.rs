#[cfg(debug_assertions)]
use elapsed::prelude::*;

#[async_trait::async_trait]
pub trait Indexer {
    async fn index(&self, documents: Vec<crate::Document>) -> Result<(), error::Error>;
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
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, documents)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn index(&self, documents: Vec<crate::Document>) -> Result<(), error::Error> {
        if documents.is_empty() {
            return Ok(());
        }
        let schema = self.schema.clone();
        let tokenizer = self.tokenizer.clone();
        let document_uuids = self.document_storage.save_bulk(&documents).await?;
        let merged = tokio::task::spawn_blocking(move || {
            let mut merged = std::collections::HashMap::<String, crate::PostingsList>::new();
            for (document, document_uuid) in documents.iter().zip(document_uuids.iter()) {
                let mut t = std::collections::HashMap::<String, crate::Posting>::new();
                for (field, value) in document.iter() {
                    if !schema
                        .get_field(field)
                        .map(|f| f.is_indexed())
                        .unwrap_or(false)
                    {
                        continue;
                    }
                    match value {
                        crate::Value::String(body) => {
                            let mut tokenizer = tokenizer.clone();
                            let body = body.clone();
                            let tokens = tokenizer.tokenize(body)?;
                            for (i, token) in tokens.iter().enumerate() {
                                if let Some(posting) = t.get_mut(token) {
                                    posting.positions.insert(i as u64);
                                } else {
                                    t.insert(
                                        token.clone(),
                                        crate::Posting::new(
                                            *document_uuid,
                                            vec![i as u64].into_iter().collect(),
                                        ),
                                    );
                                }
                            }
                        }
                    }
                }
                for (token, posting) in t {
                    let postings_list = crate::PostingsList::from_postings(vec![posting]);
                    merged
                        .entry(token)
                        .and_modify(|existing| {
                            *existing = existing.union(postings_list.clone());
                        })
                        .or_insert(postings_list);
                }
            }
            Ok::<_, error::Error>(merged)
        })
        .await
        .map_err(|e| error::error!("{e}"))??;

        let indices: Vec<crate::InvertedIndex> = merged
            .into_iter()
            .map(|(token, postings_list)| crate::InvertedIndex::new(token, postings_list))
            .collect();
        self.token_storage.save_postings_list_bulk(indices).await?;

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
            .expect_save_postings_list_bulk()
            .withf(|indices| {
                indices.len() == 1
                    && indices[0].token == "dummy"
                    && indices[0].postings_list
                        == crate::PostingsList::from_postings(vec![crate::Posting::new(
                            1,
                            std::collections::BTreeSet::from_iter(vec![0]),
                        )])
            })
            .times(1)
            .returning(|_| Box::pin(async { Ok(()) }));
        let mut bm = std::collections::BTreeMap::new();
        bm.insert(
            "content".to_string(),
            crate::Value::String("dummy".to_string()),
        );
        mock_document_storage
            .expect_save_bulk()
            .withf(|documents| {
                documents.len() == 1
                    && documents[0]
                        .get("content")
                        .map(|v| matches!(v, crate::Value::String(s) if s == "dummy"))
                        .unwrap_or(false)
            })
            .times(1)
            .returning(|_| Box::pin(async { Ok(vec![1]) }));
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

        assert!(indexer.index(vec![crate::Document(bm)]).await.is_ok());
        Ok(())
    }

    #[tokio::test]
    async fn index_with_multiple_token() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mut mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_save_postings_list_bulk()
            .withf(|indices| {
                indices.len() == 2
                    && indices.iter().any(|index| {
                        index.token == "dummy"
                            && index.postings_list
                                == crate::PostingsList::from_postings(vec![crate::Posting::new(
                                    1,
                                    std::collections::BTreeSet::from_iter(vec![0, 2]),
                                )])
                    })
                    && indices.iter().any(|index| {
                        index.token == " "
                            && index.postings_list
                                == crate::PostingsList::from_postings(vec![crate::Posting::new(
                                    1,
                                    std::collections::BTreeSet::from_iter(vec![1]),
                                )])
                    })
            })
            .times(1)
            .returning(|_| Box::pin(async { Ok(()) }));
        let mut bm = std::collections::BTreeMap::new();
        bm.insert(
            "content".to_string(),
            crate::Value::String("dummy dummy".to_string()),
        );
        mock_document_storage
            .expect_save_bulk()
            .withf(|documents| {
                documents.len() == 1
                    && documents[0]
                        .get("content")
                        .map(|v| matches!(v, crate::Value::String(s) if s == "dummy dummy"))
                        .unwrap_or(false)
            })
            .times(1)
            .returning(|_| Box::pin(async { Ok(vec![1]) }));
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

        assert!(indexer.index(vec![crate::Document(bm)]).await.is_ok());
        Ok(())
    }
}
