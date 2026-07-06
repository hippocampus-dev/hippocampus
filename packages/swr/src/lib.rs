//! Stale-While-Revalidate cache for async operations.
//!
//! Returns cached values immediately when stale, while refreshing in the background.
//! Uses [`singleflight`] to deduplicate concurrent refresh operations.
//!
//! # Cache states
//!
//! | State | Condition | Behavior |
//! |-------|-----------|----------|
//! | Fresh | `elapsed < stale_after` | Return cached value |
//! | Stale | `stale_after <= elapsed < expire_after` | Return cached value, background refresh |
//! | Expired | `elapsed >= expire_after` | Block on refresh |
//!
//! # Error handling
//!
//! | State | On success | On error |
//! |-------|------------|----------|
//! | Stale (background) | Update cache | Discard error, preserve stale value |
//! | Expired/miss | Cache and return | Return error, do NOT cache |
//!
//! # Examples
//!
//! ```rust,ignore
//! let cache: swr::Cache<String, String, MyError> = swr::Cache::new();
//!
//! let value = cache.get(
//!     &"key".to_string(),
//!     || async {
//!         let data = fetch_from_backend().await?;
//!         Ok(swr::FetchResult {
//!             value: data,
//!             stale_after: Duration::from_secs(60),
//!             expire_after: Duration::from_secs(300),
//!         })
//!     },
//! ).await?;
//! ```

pub struct FetchResult<V> {
    pub value: V,
    pub stale_after: std::time::Duration,
    pub expire_after: std::time::Duration,
}

pub struct Cache<K, V, E> {
    entries: std::sync::Arc<std::sync::Mutex<std::collections::HashMap<K, Entry<V>>>>,
    group: std::sync::Arc<singleflight::Group<K, Result<V, std::sync::Arc<E>>>>,
}

struct Entry<V> {
    value: V,
    created_at: tokio::time::Instant,
    stale_after: std::time::Duration,
    expire_after: std::time::Duration,
}

impl<K, V, E> Clone for Cache<K, V, E> {
    fn clone(&self) -> Self {
        Self {
            entries: self.entries.clone(),
            group: self.group.clone(),
        }
    }
}

impl<K, V, E> Cache<K, V, E>
where
    K: std::cmp::Eq + std::hash::Hash + Clone + Send + Sync + 'static,
    V: Clone + Send + Sync + 'static,
    E: Send + Sync + 'static,
{
    pub fn new() -> Self {
        Self {
            entries: std::sync::Arc::new(std::sync::Mutex::new(std::collections::HashMap::new())),
            group: std::sync::Arc::new(singleflight::Group::new()),
        }
    }

    pub async fn get<F, Fut>(&self, key: &K, f: F) -> Result<V, std::sync::Arc<E>>
    where
        F: FnOnce() -> Fut + Send + 'static,
        Fut: std::future::Future<Output = Result<FetchResult<V>, E>> + Send + 'static,
    {
        {
            let entries = self.entries.lock().unwrap_or_else(|e| e.into_inner());
            if let Some(entry) = entries.get(key) {
                let elapsed = entry.created_at.elapsed();

                if elapsed < entry.stale_after {
                    return Ok(entry.value.clone());
                }

                if elapsed < entry.expire_after {
                    let stale_value = entry.value.clone();
                    let entries = self.entries.clone();
                    let group = self.group.clone();
                    let key = key.clone();
                    tokio::spawn(async move {
                        let key_clone = key.clone();
                        let _ = group
                            .work(&key, || async move {
                                match f().await {
                                    Ok(result) => {
                                        let mut entries =
                                            entries.lock().unwrap_or_else(|e| e.into_inner());
                                        entries.insert(
                                            key_clone,
                                            Entry {
                                                value: result.value.clone(),
                                                created_at: tokio::time::Instant::now(),
                                                stale_after: result.stale_after,
                                                expire_after: result.expire_after,
                                            },
                                        );
                                        Ok(result.value)
                                    }
                                    Err(e) => Err(std::sync::Arc::new(e)),
                                }
                            })
                            .await;
                    });
                    return Ok(stale_value);
                }
            }
        }

        let entries = self.entries.clone();
        let key_clone = key.clone();
        let (result, _) = self
            .group
            .work(key, || async move {
                match f().await {
                    Ok(result) => {
                        let mut entries = entries.lock().unwrap_or_else(|e| e.into_inner());
                        entries.insert(
                            key_clone,
                            Entry {
                                value: result.value.clone(),
                                created_at: tokio::time::Instant::now(),
                                stale_after: result.stale_after,
                                expire_after: result.expire_after,
                            },
                        );
                        Ok(result.value)
                    }
                    Err(e) => Err(std::sync::Arc::new(e)),
                }
            })
            .await;
        result
    }
}

impl<K, V, E> std::fmt::Debug for Cache<K, V, E> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let count = self.entries.lock().unwrap_or_else(|e| e.into_inner()).len();
        f.debug_struct("Cache").field("entries", &count).finish()
    }
}

impl<K, V, E> Default for Cache<K, V, E>
where
    K: std::cmp::Eq + std::hash::Hash + Clone + Send + Sync + 'static,
    V: Clone + Send + Sync + 'static,
    E: Send + Sync + 'static,
{
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const STALE: std::time::Duration = std::time::Duration::from_secs(60);
    const EXPIRE: std::time::Duration = std::time::Duration::from_secs(300);
    const STALE_SHORT: std::time::Duration = std::time::Duration::from_millis(10);
    const EXPIRE_SHORT: std::time::Duration = std::time::Duration::from_millis(10);

    #[tokio::test]
    async fn fresh() {
        let cache: Cache<String, String, String> = Cache::new();

        let value = cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "value".to_string(),
                    stale_after: STALE,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();
        assert_eq!(value, "value");

        let call_count = std::sync::Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let call_count_clone = call_count.clone();
        let value = cache
            .get(&"key".to_string(), || async move {
                call_count_clone.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                Ok(FetchResult {
                    value: "new_value".to_string(),
                    stale_after: STALE,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();
        assert_eq!(value, "value");
        assert_eq!(call_count.load(std::sync::atomic::Ordering::SeqCst), 0);
    }

    #[tokio::test]
    async fn stale_returns_old_value_and_refreshes() {
        let cache: Cache<String, String, String> = Cache::new();

        cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "old".to_string(),
                    stale_after: STALE_SHORT,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();

        tokio::time::sleep(std::time::Duration::from_millis(20)).await;

        let value = cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "new".to_string(),
                    stale_after: STALE,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();
        assert_eq!(value, "old");

        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let value = cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "should_not_call".to_string(),
                    stale_after: STALE,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();
        assert_eq!(value, "new");
    }

    #[tokio::test]
    async fn expired() {
        let cache: Cache<String, String, String> = Cache::new();

        cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "old".to_string(),
                    stale_after: std::time::Duration::from_millis(5),
                    expire_after: EXPIRE_SHORT,
                })
            })
            .await
            .unwrap();

        tokio::time::sleep(std::time::Duration::from_millis(20)).await;

        let value = cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "refreshed".to_string(),
                    stale_after: std::time::Duration::from_millis(5),
                    expire_after: EXPIRE_SHORT,
                })
            })
            .await
            .unwrap();
        assert_eq!(value, "refreshed");
    }

    #[tokio::test]
    async fn different_keys() {
        let cache: Cache<String, String, String> = Cache::new();

        let v1 = cache
            .get(&"a".to_string(), || async {
                Ok(FetchResult {
                    value: "value_a".to_string(),
                    stale_after: STALE,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();
        let v2 = cache
            .get(&"b".to_string(), || async {
                Ok(FetchResult {
                    value: "value_b".to_string(),
                    stale_after: STALE,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();

        assert_eq!(v1, "value_a");
        assert_eq!(v2, "value_b");
    }

    #[tokio::test]
    async fn concurrent_stale_deduplication() {
        let cache = std::sync::Arc::new(Cache::<String, String, String>::new());

        cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "old".to_string(),
                    stale_after: STALE_SHORT,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();

        tokio::time::sleep(std::time::Duration::from_millis(20)).await;

        let call_count = std::sync::Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let barrier = std::sync::Arc::new(tokio::sync::Barrier::new(5));

        let mut handles = Vec::new();
        for _ in 0..5 {
            let cache = cache.clone();
            let call_count = call_count.clone();
            let barrier = barrier.clone();
            handles.push(tokio::spawn(async move {
                barrier.wait().await;
                cache
                    .get(&"key".to_string(), || async move {
                        call_count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
                        Ok(FetchResult {
                            value: "new".to_string(),
                            stale_after: STALE_SHORT,
                            expire_after: EXPIRE,
                        })
                    })
                    .await
            }));
        }

        for handle in handles {
            let value = handle.await.unwrap().unwrap();
            assert_eq!(value, "old");
        }

        tokio::time::sleep(std::time::Duration::from_millis(100)).await;
        assert_eq!(call_count.load(std::sync::atomic::Ordering::SeqCst), 1);
    }

    #[tokio::test]
    async fn expired_error_not_cached() {
        let cache: Cache<String, String, String> = Cache::new();

        let result = cache
            .get(&"key".to_string(), || async {
                Err("fetch failed".to_string())
            })
            .await;
        assert!(result.is_err());

        let value = cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "recovered".to_string(),
                    stale_after: STALE,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();
        assert_eq!(value, "recovered");
    }

    #[tokio::test]
    async fn stale_background_error_preserves_value() {
        let cache: Cache<String, String, String> = Cache::new();

        cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "good".to_string(),
                    stale_after: STALE_SHORT,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();

        tokio::time::sleep(std::time::Duration::from_millis(20)).await;

        let value = cache
            .get(&"key".to_string(), || async {
                Err("background failure".to_string())
            })
            .await
            .unwrap();
        assert_eq!(value, "good");

        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let value = cache
            .get(&"key".to_string(), || async {
                Ok(FetchResult {
                    value: "should_not_call".to_string(),
                    stale_after: STALE_SHORT,
                    expire_after: EXPIRE,
                })
            })
            .await
            .unwrap();
        assert_eq!(value, "good");
    }
}
