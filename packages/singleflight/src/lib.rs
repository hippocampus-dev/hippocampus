//! Singleflight deduplication for concurrent async calls.
//!
//! When multiple callers request the same key concurrently, only one executes the
//! underlying function while others wait and receive a clone of the result.
//!
//! # Examples
//!
//! ```rust,ignore
//! let group: singleflight::Group<String, Result<Token, std::sync::Arc<Error>>> =
//!     singleflight::Group::new();
//!
//! let (result, shared) = group.work(&key, || async {
//!     fetch_token().await
//! }).await;
//! ```

pub struct Group<K, V> {
    flights: std::sync::Mutex<std::collections::HashMap<K, std::sync::Arc<Flight<V>>>>,
}

struct Flight<V> {
    cell: tokio::sync::OnceCell<V>,
    duplicates: std::sync::atomic::AtomicUsize,
}

impl<K, V> Group<K, V>
where
    K: std::cmp::Eq + std::hash::Hash + Clone,
    V: Clone,
{
    pub fn new() -> Self {
        Self {
            flights: std::sync::Mutex::new(std::collections::HashMap::new()),
        }
    }

    pub async fn work<F, Fut>(&self, key: &K, f: F) -> (V, bool)
    where
        F: FnOnce() -> Fut,
        Fut: std::future::Future<Output = V>,
    {
        let flight = {
            let mut flights = self.flights.lock().unwrap_or_else(|e| e.into_inner());
            match flights.entry(key.clone()) {
                std::collections::hash_map::Entry::Occupied(entry) => {
                    entry
                        .get()
                        .duplicates
                        .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
                    entry.get().clone()
                }
                std::collections::hash_map::Entry::Vacant(entry) => entry
                    .insert(std::sync::Arc::new(Flight {
                        cell: tokio::sync::OnceCell::new(),
                        duplicates: std::sync::atomic::AtomicUsize::new(0),
                    }))
                    .clone(),
            }
        };

        let result = flight.cell.get_or_init(f).await.clone();
        let shared = flight.duplicates.load(std::sync::atomic::Ordering::Relaxed) > 0;

        {
            let mut flights = self.flights.lock().unwrap_or_else(|e| e.into_inner());
            if let Some(existing) = flights.get(key)
                && std::sync::Arc::ptr_eq(existing, &flight)
            {
                flights.remove(key);
            }
        }

        (result, shared)
    }
}

impl<K, V> std::fmt::Debug for Group<K, V> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let count = self.flights.lock().unwrap_or_else(|e| e.into_inner()).len();
        f.debug_struct("Group").field("flights", &count).finish()
    }
}

impl<K, V> Default for Group<K, V>
where
    K: std::cmp::Eq + std::hash::Hash + Clone,
    V: Clone,
{
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn deduplication() {
        let call_count = std::sync::Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let group: std::sync::Arc<Group<String, String>> = std::sync::Arc::new(Group::new());

        let barrier = std::sync::Arc::new(tokio::sync::Barrier::new(5));

        let mut handles = Vec::new();
        for _ in 0..5 {
            let group = group.clone();
            let call_count = call_count.clone();
            let barrier = barrier.clone();
            handles.push(tokio::spawn(async move {
                barrier.wait().await;
                group
                    .work(&"key".to_string(), || async {
                        call_count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
                        "result".to_string()
                    })
                    .await
            }));
        }

        for handle in handles {
            let (result, shared) = handle.await.unwrap();
            assert_eq!(result, "result");
            assert!(shared);
        }

        assert_eq!(call_count.load(std::sync::atomic::Ordering::SeqCst), 1);
    }

    #[tokio::test]
    async fn different_keys() {
        let call_count = std::sync::Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let group: std::sync::Arc<Group<String, String>> = std::sync::Arc::new(Group::new());

        let barrier = std::sync::Arc::new(tokio::sync::Barrier::new(2));

        let mut handles = Vec::new();
        for i in 0..2 {
            let group = group.clone();
            let call_count = call_count.clone();
            let barrier = barrier.clone();
            let key = format!("key-{i}");
            handles.push(tokio::spawn(async move {
                barrier.wait().await;
                group
                    .work(&key, || async {
                        call_count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
                        format!("result-{i}")
                    })
                    .await
            }));
        }

        let mut results: Vec<String> = Vec::new();
        for handle in handles {
            let (result, shared) = handle.await.unwrap();
            assert!(!shared);
            results.push(result);
        }

        results.sort();
        assert_eq!(results, vec!["result-0", "result-1"]);
        assert_eq!(call_count.load(std::sync::atomic::Ordering::SeqCst), 2);
    }

    #[tokio::test]
    async fn cleanup_after_completion() {
        let group: Group<String, String> = Group::new();

        let (result, shared) = group
            .work(&"key".to_string(), || async { "first".to_string() })
            .await;
        assert_eq!(result, "first");
        assert!(!shared);

        let (result, shared) = group
            .work(&"key".to_string(), || async { "second".to_string() })
            .await;
        assert_eq!(result, "second");
        assert!(!shared);
    }
}
