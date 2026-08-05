use crate::types::lru::LockFreeLruCache;
use std::sync::Arc;

#[derive(Clone)]
pub struct CachedTokenStorage<T: crate::storage::TokenStorage> {
    inner: T,
    cache: Arc<LockFreeLruCache<String, crate::PostingsList>>,
}

impl<T: crate::storage::TokenStorage> CachedTokenStorage<T> {
    pub fn new(inner: T, cache_size: usize) -> Self {
        Self {
            inner,
            cache: Arc::new(LockFreeLruCache::new(cache_size)),
        }
    }

    pub async fn cache_stats(&self) -> CacheStats {
        CacheStats {
            size: self.cache.len(),
        }
    }
}

#[async_trait::async_trait]
impl<T: Send + Sync + crate::storage::TokenStorage> crate::storage::TokenStorage
    for CachedTokenStorage<T>
{
    async fn save_postings_list(&self, index: crate::InvertedIndex) -> Result<(), error::Error> {
        self.inner.save_postings_list(index).await
    }

    #[cfg(not(test))]
    async fn get_postings_list<S: AsRef<str> + std::fmt::Debug + Send + Sync>(
        &self,
        token: S,
    ) -> Result<crate::PostingsList, error::Error> {
        let token_str = token.as_ref().to_string();

        if let Some(cached) = self.cache.get(&token_str) {
            return Ok(cached);
        }

        let result = self.inner.get_postings_list(token).await?;
        self.cache.put(token_str, result.clone());
        Ok(result)
    }

    #[cfg(test)]
    async fn get_postings_list(&self, token: &str) -> Result<crate::PostingsList, error::Error> {
        let token_str = token.to_string();

        if let Some(cached) = self.cache.get(&token_str) {
            return Ok(cached);
        }

        let result = self.inner.get_postings_list(token).await?;
        self.cache.put(token_str, result.clone());
        Ok(result)
    }

    async fn save_postings_list_bulk(
        &self,
        indices: Vec<crate::InvertedIndex>,
    ) -> Result<(), error::Error> {
        self.inner.save_postings_list_bulk(indices).await
    }
}

#[derive(Debug, Clone)]
pub struct CacheStats {
    pub size: usize,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::storage::{MockTokenStorage, TokenStorage};
    use crate::{Posting, PostingsList};
    use std::collections::BTreeSet;
    use std::sync::Arc;

    #[tokio::test]
    async fn test_cache_hit() {
        let mut mock = MockTokenStorage::new();

        mock.expect_get_postings_list()
            .with(mockall::predicate::eq("test"))
            .times(1) // Should only be called once due to caching
            .returning(|_| {
                Box::pin(async {
                    Ok(PostingsList::from_postings(vec![Posting::new(
                        1,
                        BTreeSet::from_iter(vec![0, 1]),
                    )]))
                })
            });

        let cached_storage = CachedTokenStorage::new(mock, 10);

        let result1 = cached_storage.get_postings_list("test").await.unwrap();
        let result2 = cached_storage.get_postings_list("test").await.unwrap();

        assert_eq!(result1.len(), result2.len());
        assert_eq!(cached_storage.cache_stats().await.size, 1);
    }

    #[tokio::test]
    async fn test_cache_eviction() {
        let mut mock = MockTokenStorage::new();

        // Use capacity * 2 tokens to ensure eviction occurs even with shard distribution
        // LRU cache uses 16 shards, so capacity=32 gives capacity_per_shard=2
        const CAPACITY: usize = 32;
        const TOKEN_COUNT: usize = CAPACITY * 2;

        for i in 0..TOKEN_COUNT {
            let token = format!("token{i}");
            mock.expect_get_postings_list()
                .withf(move |t: &str| t == token.as_str())
                .returning(move |_| {
                    let i = i;
                    Box::pin(async move {
                        Ok(PostingsList::from_postings(vec![Posting::new(
                            i as u64,
                            BTreeSet::from_iter(vec![i as u64]),
                        )]))
                    })
                });
        }

        let cached_storage = CachedTokenStorage::new(mock, CAPACITY);

        for i in 0..TOKEN_COUNT {
            cached_storage
                .get_postings_list(&format!("token{i}"))
                .await
                .unwrap();
        }

        let cache_size = cached_storage.cache_stats().await.size;
        assert!(cache_size < TOKEN_COUNT);
        assert!(cache_size <= CAPACITY);
    }

    #[tokio::test]
    async fn test_concurrent_cache_access() {
        let mut mock = MockTokenStorage::new();

        mock.expect_get_postings_list()
            .with(mockall::predicate::eq("concurrent"))
            .times(1) // Should only be called once despite concurrent access
            .returning(|_| {
                Box::pin(async {
                    Ok(PostingsList::from_postings(vec![Posting::new(
                        1,
                        BTreeSet::from_iter(vec![0]),
                    )]))
                })
            });

        let cached_storage = Arc::new(CachedTokenStorage::new(mock, 10));
        let mut handles = vec![];

        for _ in 0..10 {
            let storage = cached_storage.clone();
            let handle =
                tokio::spawn(async move { storage.get_postings_list("concurrent").await.unwrap() });
            handles.push(handle);
        }

        for handle in handles {
            handle.await.unwrap();
        }

        assert_eq!(cached_storage.cache_stats().await.size, 1);
    }
}
