use futures::{StreamExt, TryStreamExt};

#[cfg(debug_assertions)]
use elapsed::prelude::*;

#[async_trait::async_trait]
pub trait Searcher {
    async fn search(
        &self,
        query: &hippocampusql::Query,
        option: SearchOption,
    ) -> Result<Vec<SearchResult>, error::Error>;
}

#[derive(Clone, Debug)]
pub struct SearchOption {
    pub offset: usize,
    pub page_size: usize,
}

impl Default for SearchOption {
    fn default() -> Self {
        Self {
            offset: 0,
            page_size: 10,
        }
    }
}

#[derive(Clone, Debug)]
pub struct DocumentSearcher<
    DS: Send + Sync + crate::storage::DocumentStorage,
    TS: Send + Sync + crate::storage::TokenStorage,
    T: Clone + Send + Sync + crate::tokenizer::Tokenizer,
    S: Clone + Send + Sync + crate::scorer::Scorer,
> {
    document_storage: DS,
    token_storage: TS,
    tokenizer: T,
    scorer: S,
    schema: crate::Schema,
    fragments_size: usize,
    fragments_offset: usize,
}

impl<
    DS: Send + Sync + crate::storage::DocumentStorage,
    TS: Send + Sync + crate::storage::TokenStorage,
    T: Clone + Send + Sync + crate::tokenizer::Tokenizer,
    S: Clone + Send + Sync + crate::scorer::Scorer,
> DocumentSearcher<DS, TS, T, S>
{
    pub fn new(
        document_storage: DS,
        token_storage: TS,
        tokenizer: T,
        scorer: S,
        schema: crate::Schema,
    ) -> DocumentSearcher<DS, TS, T, S> {
        Self {
            document_storage,
            token_storage,
            tokenizer,
            scorer,
            schema,
            fragments_size: 5,
            fragments_offset: 10,
        }
    }

    #[cfg_attr(
        feature = "tracing",
        tracing::instrument(skip(self, token_associated_postings_list_list))
    )]
    #[cfg_attr(debug_assertions, elapsed)]
    fn intersect_postings_list(
        &self,
        token_associated_postings_list_list: &[TokenAssociatedPostingsList],
        phrased: bool,
    ) -> crate::PostingsList {
        let mut postings_list = Vec::new();

        let mut cursors: Vec<usize> = vec![0; token_associated_postings_list_list.len()];

        let mut first_cursor = 0;
        let first_token_postings_list_size =
            token_associated_postings_list_list[0].postings_list.len();
        let first_token_position = &token_associated_postings_list_list[0].token_position;
        // postings_list loop
        while first_cursor < first_token_postings_list_size {
            let first_posting = &token_associated_postings_list_list[0].postings_list[first_cursor];
            let mut matches = if phrased {
                first_posting
                    .positions
                    .iter()
                    .filter_map(|position| position.checked_sub(*first_token_position))
                    .collect()
            } else {
                first_posting.positions.clone()
            };
            let mut i = 1;
            // token_associated_postings_list loop
            'first: while i < token_associated_postings_list_list.len() {
                let next_token_postings_list_size =
                    token_associated_postings_list_list[i].postings_list.len();
                let next_token_position = &token_associated_postings_list_list[i].token_position;
                // postings_list loop
                'next: while cursors[i] < next_token_postings_list_size {
                    let next_posting =
                        &token_associated_postings_list_list[i].postings_list[cursors[i]];
                    if first_posting.document_uuid < next_posting.document_uuid {
                        break 'first;
                    }
                    if first_posting.document_uuid == next_posting.document_uuid {
                        if phrased {
                            matches = matches
                                .intersection(
                                    &next_posting
                                        .positions
                                        .iter()
                                        .filter_map(|position| {
                                            position.checked_sub(*next_token_position)
                                        })
                                        .collect(),
                                )
                                .cloned()
                                .collect();
                        } else {
                            matches = matches.union(&next_posting.positions).cloned().collect();
                        }
                        break 'next;
                    }
                    cursors[i] += 1;
                }
                i += 1;
            }
            if i == token_associated_postings_list_list.len() && !(phrased && matches.is_empty()) {
                postings_list.push(crate::Posting::new(
                    first_posting.document_uuid,
                    matches.clone(),
                ));
            }
            first_cursor += 1;
        }

        crate::PostingsList::from_postings(postings_list)
    }

    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, query)))]
    #[cfg_attr(debug_assertions, elapsed)]
    fn eval<'a>(
        &'a self,
        query: &'a hippocampusql::Query,
    ) -> futures::future::Either<
        futures::future::Ready<Result<crate::PostingsList, error::Error>>,
        futures::future::BoxFuture<'a, Result<crate::PostingsList, error::Error>>,
    > {
        match query {
            hippocampusql::Query::Term(term) => match self.tokenizer.clone().tokenize(&term.0) {
                Ok(tokens) if tokens.is_empty() => futures::future::Either::Left(
                    futures::future::ready(Ok(crate::PostingsList::new())),
                ),
                Ok(_) => futures::future::Either::Right(Box::pin(async move {
                    let mut token_associated_postings_list_list: Vec<TokenAssociatedPostingsList> =
                        Vec::new();
                    for (i, token) in self.tokenizer.clone().tokenize(&term.0)?.iter().enumerate() {
                        if let Ok(postings_list) = self.token_storage.get_postings_list(token).await
                        {
                            token_associated_postings_list_list.push(
                                TokenAssociatedPostingsList::new(
                                    i as u64,
                                    postings_list.len() as u64,
                                    postings_list,
                                ),
                            );
                        }
                    }

                    if token_associated_postings_list_list.is_empty() {
                        return Ok(crate::PostingsList::new());
                    }

                    token_associated_postings_list_list
                        .sort_by(|x, y| x.postings_list.len().cmp(&y.postings_list.len()));

                    Ok(self.intersect_postings_list(&token_associated_postings_list_list, false))
                })),
                Err(e) => futures::future::Either::Left(futures::future::ready(Err(e))),
            },
            hippocampusql::Query::Phrase(phrase) => {
                match self.tokenizer.clone().tokenize(phrase.0.to_string()) {
                    Ok(tokens) if tokens.is_empty() => futures::future::Either::Left(
                        futures::future::ready(Ok(crate::PostingsList::new())),
                    ),
                    Ok(_) => {
                        futures::future::Either::Right(Box::pin(async move {
                            let mut token_associated_postings_list_list: Vec<
                                TokenAssociatedPostingsList,
                            > = Vec::new();
                            for (i, token) in self
                                .tokenizer
                                .clone()
                                .tokenize(phrase.0.to_string())?
                                .iter()
                                .enumerate()
                            {
                                if let Ok(postings_list) =
                                    self.token_storage.get_postings_list(token).await
                                {
                                    token_associated_postings_list_list.push(
                                        TokenAssociatedPostingsList::new(
                                            i as u64,
                                            postings_list.len() as u64,
                                            postings_list,
                                        ),
                                    );
                                }
                            }

                            if token_associated_postings_list_list.is_empty() {
                                return Ok(crate::PostingsList::new());
                            }

                            token_associated_postings_list_list
                                .sort_by(|x, y| x.postings_list.len().cmp(&y.postings_list.len()));

                            Ok(self.intersect_postings_list(
                                &token_associated_postings_list_list,
                                true,
                            ))
                        }))
                    }
                    Err(e) => futures::future::Either::Left(futures::future::ready(Err(e))),
                }
            }
            hippocampusql::Query::Operation(operation) => match operation.operator {
                hippocampusql::Operator::OR => {
                    futures::future::Either::Right(Box::pin(async move {
                        let left = self.eval(&operation.left).await?;
                        let right = self.eval(&operation.right).await?;
                        Ok(left.union(right))
                    }))
                }
                hippocampusql::Operator::AND => {
                    futures::future::Either::Right(Box::pin(async move {
                        let left = self.eval(&operation.left).await?;
                        let right = self.eval(&operation.right).await?;
                        Ok(left.intersection(right))
                    }))
                }
                hippocampusql::Operator::NOT => {
                    futures::future::Either::Right(Box::pin(async move {
                        let left = self.eval(&operation.left).await?;
                        let right = self.eval(&operation.right).await?;
                        Ok(left.difference(right))
                    }))
                }
            },
        }
    }
}

#[async_trait::async_trait]
impl<
    DS: Send + Sync + crate::storage::DocumentStorage,
    TS: Send + Sync + crate::storage::TokenStorage,
    T: Clone + Send + Sync + crate::tokenizer::Tokenizer,
    S: Clone + Send + Sync + crate::scorer::Scorer,
> Searcher for DocumentSearcher<DS, TS, T, S>
{
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, query)))]
    #[cfg_attr(debug_assertions, elapsed)]
    async fn search(
        &self,
        query: &hippocampusql::Query,
        option: SearchOption,
    ) -> Result<Vec<SearchResult>, error::Error> {
        let postings_list = self.eval(query).await?;
        let postings_list_len = postings_list.len() as i64;
        let stream = futures::stream::iter(postings_list)
            .skip(option.offset)
            .take(option.page_size)
            .map(|posting| async move {
                let score = self.scorer.calculate(&crate::scorer::Parameter {
                    documents_count: postings_list_len,
                    positions_count: posting.positions.len() as i64,
                })?;
                let document = self.document_storage.find(posting.document_uuid).await?;
                let mut fragments = std::collections::HashSet::new();
                for (field, value) in document.iter() {
                    if !self.schema.get_field(field).unwrap().is_indexed() {
                        continue;
                    }
                    match value {
                        crate::Value::String(body) => {
                            let tokens = self.tokenizer.clone().tokenize(body)?;
                            let f = posting
                                .positions
                                .clone()
                                .into_iter()
                                .take(self.fragments_size)
                                .map(|position| {
                                    let position = position as usize;
                                    let begin = position.saturating_sub(self.fragments_offset);
                                    let end = if tokens.len() > position + self.fragments_offset {
                                        position + self.fragments_offset
                                    } else {
                                        tokens.len()
                                    };
                                    tokens[begin..end].join("")
                                })
                                .collect::<std::collections::HashSet<String>>();
                            fragments.extend(f)
                        }
                    }
                }
                Ok(SearchResult::new(document, fragments, score as i64))
            });

        let results: Result<Vec<SearchResult>, error::Error> = stream
            .buffer_unordered(option.page_size)
            .try_collect()
            .await;

        let mut results = results?;
        results.sort_by(|x, y| y.score.cmp(&x.score));
        Ok(results)
    }
}

#[derive(Clone, Debug)]
struct TokenAssociatedPostingsList {
    token_position: u64,
    postings_list: crate::PostingsList,
}

impl TokenAssociatedPostingsList {
    fn new(token_position: u64, _documents_count: u64, postings_list: crate::PostingsList) -> Self {
        Self {
            token_position,
            postings_list,
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SearchResult {
    pub document: crate::Document,
    pub fragments: std::collections::HashSet<String>,
    pub score: i64,
}

impl SearchResult {
    fn new(
        document: crate::Document,
        fragments: std::collections::HashSet<String>,
        score: i64,
    ) -> Self {
        SearchResult {
            document,
            fragments,
            score,
        }
    }
}

#[cfg(test)]
mod tests {
    use std::iter::FromIterator;

    use super::*;

    #[tokio::test]
    async fn search() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mut mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("dummy"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![4])),
                        crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![4])),
                    ]))
                })
            });
        let mut bm1 = std::collections::BTreeMap::new();
        bm1.insert(
            "content".to_string(),
            crate::Value::String("this is dummy".to_string()),
        );
        let document1 = crate::Document(bm1);
        let cloned_document1 = document1.clone();
        mock_document_storage
            .expect_find()
            .with(mockall::predicate::eq(1))
            .times(1)
            .returning(move |_| {
                let cloned_document1 = cloned_document1.clone();
                Box::pin(async move { Ok(cloned_document1) })
            });
        let mut bm2 = std::collections::BTreeMap::new();
        bm2.insert(
            "content".to_string(),
            crate::Value::String("that is dummy".to_string()),
        );
        let document2 = crate::Document(bm2);
        let cloned_document2 = document2.clone();
        mock_document_storage
            .expect_find()
            .with(mockall::predicate::eq(2))
            .times(1)
            .returning(move |_| {
                let cloned_document2 = cloned_document2.clone();
                Box::pin(async move { Ok(cloned_document2) })
            });
        let mut schema = crate::Schema::new();
        schema.add_field(crate::Field {
            name: "content".to_string(),
            field_type: crate::FieldType::String(crate::StringOption { indexeing: true }),
        });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(2),
            schema,
        );

        let mut first_fragments = std::collections::HashSet::new();
        first_fragments.insert("this is dummy".to_string());
        let mut second_fragments = std::collections::HashSet::new();
        second_fragments.insert("that is dummy".to_string());
        assert_eq!(
            searcher
                .search(
                    &hippocampusql::Query::Term(hippocampusql::Term("dummy".to_string())),
                    SearchOption::default()
                )
                .await?,
            vec![
                SearchResult::new(document1, first_fragments.clone(), 1),
                SearchResult::new(document2, second_fragments.clone(), 1)
            ],
        );
        Ok(())
    }

    #[tokio::test]
    async fn search_with_multiple_fragments() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mut mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("match"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![2, 18])),
                    ]))
                })
            });
        let mut bm = std::collections::BTreeMap::new();
        bm.insert(
            "content".to_string(),
            crate::Value::String(
                "dummy match dummy dummy dummy dummy dummy dummy dummy match".to_string(),
            ),
        );
        let document = crate::Document(bm);
        let cloned_document = document.clone();
        mock_document_storage
            .expect_find()
            .with(mockall::predicate::eq(1))
            .times(1)
            .returning(move |_| {
                let cloned_document = cloned_document.clone();
                Box::pin(async move { Ok(cloned_document) })
            });
        let mut schema = crate::Schema::new();
        schema.add_field(crate::Field {
            name: "content".to_string(),
            field_type: crate::FieldType::String(crate::StringOption { indexeing: true }),
        });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(1),
            schema,
        );

        let mut fragments = std::collections::HashSet::new();
        fragments.insert("dummy match dummy dummy dummy dummy ".to_string());
        fragments.insert("dummy dummy dummy dummy dummy match".to_string());
        assert_eq!(
            searcher
                .search(
                    &hippocampusql::Query::Term(hippocampusql::Term("match".to_string())),
                    SearchOption::default()
                )
                .await?,
            vec![SearchResult::new(document, fragments.clone(), 2),],
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_single_token() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("dummy"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Term(hippocampusql::Term(
                    "dummy".to_string()
                )))
                .await?,
            crate::PostingsList::from_postings(vec![crate::Posting::new(
                1,
                std::collections::BTreeSet::from_iter(vec![0])
            )]),
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_multiple_token() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("multiple"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq(" "))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("string"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Term(hippocampusql::Term(
                    "multiple string".to_string()
                )))
                .await?,
            crate::PostingsList::from_postings(vec![crate::Posting::new(
                1,
                std::collections::BTreeSet::from_iter(vec![0, 1, 2])
            )]),
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_notfound() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("notfound"))
            .times(1)
            .returning(|_| Box::pin(async { error::bail!() }));
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Term(hippocampusql::Term(
                    "notfound".to_string()
                )))
                .await?,
            crate::PostingsList::new(),
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_phrase() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("multiple"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq(" "))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("string"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Phrase(hippocampusql::Phrase(
                    hippocampusql::Term("multiple string".to_string())
                )))
                .await?,
            crate::PostingsList::from_postings(vec![crate::Posting::new(
                1,
                std::collections::BTreeSet::from_iter(vec![0])
            )]),
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_phrase_but_different_document_uuid() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("multiple"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq(" "))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("string"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![2])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Phrase(hippocampusql::Phrase(
                    hippocampusql::Term("multiple string".to_string())
                )))
                .await?,
            crate::PostingsList::new(),
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_or() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("foo"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("bar"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        let mut result = Vec::from_iter(
            searcher
                .eval(&hippocampusql::Query::Operation(Box::new(
                    hippocampusql::Operation {
                        operator: hippocampusql::Operator::OR,
                        left: hippocampusql::Query::Term(hippocampusql::Term("foo".to_string())),
                        right: hippocampusql::Query::Term(hippocampusql::Term("bar".to_string())),
                    },
                )))
                .await?,
        );
        result.sort_by(|a, b| a.document_uuid.cmp(&b.document_uuid));
        assert_eq!(
            crate::PostingsList::from_postings(result),
            crate::PostingsList::from_postings(vec![
                crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![0]))
            ])
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_or_same_document_uuid() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("foo"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("bar"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Operation(Box::new(
                    hippocampusql::Operation {
                        operator: hippocampusql::Operator::OR,
                        left: hippocampusql::Query::Term(hippocampusql::Term("foo".to_string())),
                        right: hippocampusql::Query::Term(hippocampusql::Term("bar".to_string())),
                    }
                )))
                .await?,
            crate::PostingsList::from_postings(vec![crate::Posting::new(
                1,
                std::collections::BTreeSet::from_iter(vec![0, 1])
            )])
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_and() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("foo"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("bar"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                        crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Operation(Box::new(
                    hippocampusql::Operation {
                        operator: hippocampusql::Operator::AND,
                        left: hippocampusql::Query::Term(hippocampusql::Term("foo".to_string())),
                        right: hippocampusql::Query::Term(hippocampusql::Term("bar".to_string())),
                    }
                )))
                .await?,
            crate::PostingsList::from_postings(vec![crate::Posting::new(
                1,
                std::collections::BTreeSet::from_iter(vec![0, 1])
            )])
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_and_different_document_uuid() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("foo"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("bar"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Operation(Box::new(
                    hippocampusql::Operation {
                        operator: hippocampusql::Operator::AND,
                        left: hippocampusql::Query::Term(hippocampusql::Term("foo".to_string())),
                        right: hippocampusql::Query::Term(hippocampusql::Term("bar".to_string())),
                    }
                )))
                .await?,
            crate::PostingsList::new(),
        );
        Ok(())
    }

    #[tokio::test]
    async fn eval_not() -> Result<(), error::Error> {
        let mut mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("foo"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![0])),
                        crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![0])),
                    ]))
                })
            });
        mock_token_storage
            .expect_get_postings_list()
            .with(mockall::predicate::eq("bar"))
            .times(1)
            .returning(|_| {
                Box::pin(async {
                    Ok(crate::PostingsList::from_postings(vec![
                        crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                    ]))
                })
            });
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher
                .eval(&hippocampusql::Query::Operation(Box::new(
                    hippocampusql::Operation {
                        operator: hippocampusql::Operator::NOT,
                        left: hippocampusql::Query::Term(hippocampusql::Term("foo".to_string())),
                        right: hippocampusql::Query::Term(hippocampusql::Term("bar".to_string())),
                    }
                )))
                .await?,
            crate::PostingsList::from_postings(vec![crate::Posting::new(
                2,
                std::collections::BTreeSet::from_iter(vec![0])
            )])
        );
        Ok(())
    }

    #[test]
    fn intersect_postings_list() -> Result<(), error::Error> {
        let mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher.intersect_postings_list(
                &[
                    TokenAssociatedPostingsList::new(
                        0,
                        1,
                        crate::PostingsList::from_postings(vec![
                            crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                            crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![3])),
                            crate::Posting::new(3, std::collections::BTreeSet::from_iter(vec![3]))
                        ]),
                    ),
                    TokenAssociatedPostingsList::new(
                        1,
                        1,
                        crate::PostingsList::from_postings(vec![
                            crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
                            crate::Posting::new(3, std::collections::BTreeSet::from_iter(vec![4]))
                        ]),
                    ),
                    TokenAssociatedPostingsList::new(
                        2,
                        1,
                        crate::PostingsList::from_postings(vec![
                            crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
                            crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![2])),
                            crate::Posting::new(3, std::collections::BTreeSet::from_iter(vec![5])),
                        ]),
                    )
                ],
                false
            ),
            crate::PostingsList::from_postings(vec![
                crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![1, 2, 3])),
                crate::Posting::new(3, std::collections::BTreeSet::from_iter(vec![3, 4, 5]))
            ]),
        );
        Ok(())
    }

    #[test]
    fn phrased_intersect_postings_list() -> Result<(), error::Error> {
        let mock_token_storage = crate::storage::MockTokenStorage::new();
        let mock_document_storage = crate::storage::MockDocumentStorage::new();
        let searcher = DocumentSearcher::new(
            mock_document_storage,
            mock_token_storage,
            crate::tokenizer::lindera::Lindera::new()?,
            crate::scorer::tf_idf::TfIdf::new(0),
            crate::Schema::new(),
        );

        assert_eq!(
            searcher.intersect_postings_list(
                &[
                    TokenAssociatedPostingsList::new(
                        0,
                        1,
                        crate::PostingsList::from_postings(vec![
                            crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
                            crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![3])),
                            crate::Posting::new(3, std::collections::BTreeSet::from_iter(vec![3]))
                        ]),
                    ),
                    TokenAssociatedPostingsList::new(
                        1,
                        1,
                        crate::PostingsList::from_postings(vec![
                            crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
                            crate::Posting::new(3, std::collections::BTreeSet::from_iter(vec![4]))
                        ]),
                    ),
                    TokenAssociatedPostingsList::new(
                        2,
                        1,
                        crate::PostingsList::from_postings(vec![
                            crate::Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
                            crate::Posting::new(2, std::collections::BTreeSet::from_iter(vec![2])),
                            crate::Posting::new(3, std::collections::BTreeSet::from_iter(vec![5]))
                        ]),
                    )
                ],
                true
            ),
            crate::PostingsList::from_postings(vec![crate::Posting::new(
                3,
                std::collections::BTreeSet::from_iter(vec![3])
            )]),
        );
        Ok(())
    }
}
