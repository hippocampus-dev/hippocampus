pub struct BloomFilter {
    bits: std::sync::Arc<Vec<std::sync::atomic::AtomicU64>>,
    num_hash_functions: u32,
    size: usize,
    inserted_items: std::sync::atomic::AtomicUsize,
}

impl BloomFilter {
    pub fn new(expected_items: usize, false_positive_rate: f64) -> Self {
        let size = Self::optimal_size(expected_items, false_positive_rate);
        let num_hash_functions = Self::optimal_hash_functions(size, expected_items);

        let num_u64s = size.div_ceil(64);
        let mut bits = Vec::with_capacity(num_u64s);
        for _ in 0..num_u64s {
            bits.push(std::sync::atomic::AtomicU64::new(0));
        }

        Self {
            bits: std::sync::Arc::new(bits),
            num_hash_functions,
            size,
            inserted_items: std::sync::atomic::AtomicUsize::new(0),
        }
    }

    fn optimal_size(expected_items: usize, false_positive_rate: f64) -> usize {
        let ln2_squared = std::f64::consts::LN_2 * std::f64::consts::LN_2;
        let size = -(expected_items as f64) * false_positive_rate.ln() / ln2_squared;
        size.ceil() as usize
    }

    fn optimal_hash_functions(size: usize, expected_items: usize) -> u32 {
        let ratio = size as f64 / expected_items as f64;
        let k = ratio * std::f64::consts::LN_2;
        k.round().max(1.0) as u32
    }

    fn hash_with_seed<T: std::hash::Hash>(item: &T, seed: u64) -> u64 {
        let mut hasher = std::collections::hash_map::DefaultHasher::new();
        std::hash::Hasher::write_u64(&mut hasher, seed);
        item.hash(&mut hasher);
        std::hash::Hasher::finish(&hasher)
    }

    fn get_bit_positions<T: std::hash::Hash>(&self, item: &T) -> Vec<usize> {
        let mut positions = Vec::with_capacity(self.num_hash_functions as usize);

        let h1 = Self::hash_with_seed(item, 0);
        let h2 = Self::hash_with_seed(item, h1);

        for i in 0..self.num_hash_functions {
            let hash = h1.wrapping_add((i as u64).wrapping_mul(h2));
            positions.push((hash % self.size as u64) as usize);
        }

        positions
    }

    pub fn insert<T: std::hash::Hash>(&self, item: &T) {
        for bit_pos in self.get_bit_positions(item) {
            let word_index = bit_pos / 64;
            let bit_index = bit_pos % 64;
            let mask = 1u64 << bit_index;

            self.bits[word_index].fetch_or(mask, std::sync::atomic::Ordering::Relaxed);
        }
        self.inserted_items
            .fetch_add(1, std::sync::atomic::Ordering::Relaxed);
    }

    pub fn contains<T: std::hash::Hash>(&self, item: &T) -> bool {
        for bit_pos in self.get_bit_positions(item) {
            let word_index = bit_pos / 64;
            let bit_index = bit_pos % 64;
            let mask = 1u64 << bit_index;

            if self.bits[word_index].load(std::sync::atomic::Ordering::Relaxed) & mask == 0 {
                return false;
            }
        }
        true
    }

    pub fn clear(&self) {
        for word in self.bits.iter() {
            word.store(0, std::sync::atomic::Ordering::Relaxed);
        }
        self.inserted_items
            .store(0, std::sync::atomic::Ordering::Relaxed);
    }

    pub fn estimated_false_positive_rate(&self) -> f64 {
        let set_bits = self.count_set_bits();
        let p = set_bits as f64 / self.size as f64;
        p.powi(self.num_hash_functions as i32)
    }

    fn count_set_bits(&self) -> usize {
        self.bits
            .iter()
            .map(|word| word.load(std::sync::atomic::Ordering::Relaxed).count_ones() as usize)
            .sum()
    }

    pub fn estimated_items(&self) -> usize {
        self.inserted_items
            .load(std::sync::atomic::Ordering::Relaxed)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_operations() {
        let filter = BloomFilter::new(1000, 0.01);

        filter.insert(&"hello");
        filter.insert(&"world");

        assert!(filter.contains(&"hello"));
        assert!(filter.contains(&"world"));
        assert!(!filter.contains(&"foo"));
    }

    #[test]
    fn test_false_positive_rate() {
        let filter = BloomFilter::new(10000, 0.01);

        for i in 0..10000 {
            filter.insert(&i);
        }

        let mut false_positives = 0;
        for i in 10000..20000 {
            if filter.contains(&i) {
                false_positives += 1;
            }
        }

        let actual_rate = false_positives as f64 / 10000.0;
        assert!(actual_rate < 0.02);
    }

    #[test]
    fn test_clear() {
        let filter = BloomFilter::new(100, 0.01);

        filter.insert(&"test");
        assert!(filter.contains(&"test"));

        filter.clear();
        assert!(!filter.contains(&"test"));
    }

    #[test]
    fn test_concurrent_operations() {
        let filter = std::sync::Arc::new(BloomFilter::new(10000, 0.01));
        let mut handles = vec![];

        for thread_id in 0..10 {
            let filter_clone = filter.clone();
            let handle = std::thread::spawn(move || {
                for i in 0..1000 {
                    let value = thread_id * 1000 + i;
                    filter_clone.insert(&value);
                }
            });
            handles.push(handle);
        }

        for handle in handles {
            handle.join().unwrap();
        }

        for i in 0..10000 {
            assert!(filter.contains(&i));
        }
    }
}
