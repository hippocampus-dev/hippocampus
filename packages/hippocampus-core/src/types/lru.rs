use std::hash::Hasher;

const NUM_SHARDS: usize = 16;

pub struct LockFreeLruCache<K, V> {
    shards: Vec<std::sync::Arc<CacheShard<K, V>>>,
    capacity_per_shard: usize,
}

struct CacheShard<K, V> {
    map: std::sync::RwLock<std::collections::HashMap<K, std::sync::Arc<CacheEntry<V>>>>,
    size: std::sync::atomic::AtomicUsize,
    eviction_lock: std::sync::Mutex<()>,
    access_counter: std::sync::atomic::AtomicU64,
}

struct CacheEntry<V> {
    value: V,
    last_access: std::sync::atomic::AtomicU64,
}

impl<K, V> LockFreeLruCache<K, V>
where
    K: Clone + Eq + std::hash::Hash + Send + Sync + 'static,
    V: Clone + Send + Sync + 'static,
{
    pub fn new(capacity: usize) -> Self {
        assert!(capacity > 0, "Capacity must be positive");
        let capacity_per_shard = capacity.div_ceil(NUM_SHARDS);

        let mut shards = Vec::with_capacity(NUM_SHARDS);
        for _ in 0..NUM_SHARDS {
            shards.push(std::sync::Arc::new(CacheShard {
                map: std::sync::RwLock::new(std::collections::HashMap::with_capacity(
                    capacity_per_shard,
                )),
                size: std::sync::atomic::AtomicUsize::new(0),
                eviction_lock: std::sync::Mutex::new(()),
                access_counter: std::sync::atomic::AtomicU64::new(0),
            }));
        }

        Self {
            shards,
            capacity_per_shard,
        }
    }

    #[inline]
    fn get_shard(&self, key: &K) -> &std::sync::Arc<CacheShard<K, V>> {
        let mut hasher = rustc_hash::FxHasher::default();
        key.hash(&mut hasher);
        let hash = hasher.finish();
        &self.shards[(hash as usize) % NUM_SHARDS]
    }

    pub fn get(&self, key: &K) -> Option<V> {
        let shard = self.get_shard(key);

        let map = shard
            .map
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());

        if let Some(entry) = map.get(key) {
            let access_time = shard
                .access_counter
                .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
            entry
                .last_access
                .store(access_time, std::sync::atomic::Ordering::Relaxed);
            Some(entry.value.clone())
        } else {
            None
        }
    }

    pub fn put(&self, key: K, value: V) {
        let shard = self.get_shard(&key);

        {
            let map = shard
                .map
                .read()
                .unwrap_or_else(|poisoned| poisoned.into_inner());

            if map.contains_key(&key) {
                drop(map);
                let mut map = shard
                    .map
                    .write()
                    .unwrap_or_else(|poisoned| poisoned.into_inner());

                if let Some(entry) = map.get_mut(&key) {
                    let access_time = shard
                        .access_counter
                        .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
                    *entry = std::sync::Arc::new(CacheEntry {
                        value,
                        last_access: std::sync::atomic::AtomicU64::new(access_time),
                    });
                    return;
                }
            }
        }

        let access_time = shard
            .access_counter
            .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        let entry = std::sync::Arc::new(CacheEntry {
            value,
            last_access: std::sync::atomic::AtomicU64::new(access_time),
        });

        let current_size = shard.size.load(std::sync::atomic::Ordering::Relaxed);
        if current_size >= self.capacity_per_shard {
            let _eviction_guard = shard
                .eviction_lock
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner());

            if shard.size.load(std::sync::atomic::Ordering::Relaxed) >= self.capacity_per_shard {
                let mut map = shard
                    .map
                    .write()
                    .unwrap_or_else(|poisoned| poisoned.into_inner());

                let mut oldest_key = None;
                let mut oldest_time = u64::MAX;

                for (key, entry) in map.iter() {
                    let access_time = entry.last_access.load(std::sync::atomic::Ordering::Relaxed);
                    if access_time < oldest_time {
                        oldest_time = access_time;
                        oldest_key = Some(key.clone());
                    }
                }

                if let Some(key) = oldest_key {
                    map.remove(&key);
                    shard
                        .size
                        .fetch_sub(1, std::sync::atomic::Ordering::Relaxed);
                }
            }
        }

        let mut map = shard
            .map
            .write()
            .unwrap_or_else(|poisoned| poisoned.into_inner());

        if map.insert(key, entry).is_none() {
            shard
                .size
                .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        }
    }

    pub fn len(&self) -> usize {
        self.shards
            .iter()
            .map(|shard| shard.size.load(std::sync::atomic::Ordering::Relaxed))
            .sum()
    }

    pub fn clear(&self) {
        for shard in &self.shards {
            let mut map = shard
                .map
                .write()
                .unwrap_or_else(|poisoned| poisoned.into_inner());
            map.clear();
            shard.size.store(0, std::sync::atomic::Ordering::Relaxed);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_operations() {
        let cache = LockFreeLruCache::new(32);

        cache.put("key1", "value1");
        cache.put("key2", "value2");

        assert_eq!(cache.get(&"key1"), Some("value1"));
        assert_eq!(cache.get(&"key2"), Some("value2"));
        assert_eq!(cache.len(), 2);
    }

    #[test]
    fn test_eviction() {
        let cache = LockFreeLruCache::new(32);

        for i in 0..50 {
            cache.put(i, i * 2);
        }

        assert!(cache.len() <= 40); // 32 + margin for shard distribution
    }

    #[test]
    fn test_concurrent_access() {
        let cache = std::sync::Arc::new(LockFreeLruCache::new(1000));

        for i in 0..100 {
            cache.put(i, i * 2);
        }

        let mut handles = vec![];

        for _ in 0..10 {
            let cache_clone = cache.clone();
            let handle = std::thread::spawn(move || {
                for i in 0..100 {
                    assert_eq!(cache_clone.get(&i), Some(i * 2));
                }
            });
            handles.push(handle);
        }

        for handle in handles {
            handle.join().unwrap();
        }
    }
}
