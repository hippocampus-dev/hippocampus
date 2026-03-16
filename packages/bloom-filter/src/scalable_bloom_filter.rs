pub struct ScalableBloomFilter {
    filters: std::sync::RwLock<Vec<crate::BloomFilter>>,
    initial_capacity: usize,
    growth_factor: f64,
    target_false_positive_rate: f64,
    tightening_ratio: f64,
}

impl ScalableBloomFilter {
    pub fn new(
        initial_capacity: usize,
        target_false_positive_rate: f64,
        growth_factor: Option<f64>,
        tightening_ratio: Option<f64>,
    ) -> Self {
        let growth_factor = growth_factor.unwrap_or(2.0);
        let tightening_ratio = tightening_ratio.unwrap_or(0.9);

        let first_filter = crate::BloomFilter::new(initial_capacity, target_false_positive_rate);
        let filters = vec![first_filter];

        Self {
            filters: std::sync::RwLock::new(filters),
            initial_capacity,
            growth_factor,
            target_false_positive_rate,
            tightening_ratio,
        }
    }

    pub fn insert<T: std::hash::Hash>(&self, item: &T) {
        let mut filters = self
            .filters
            .write()
            .unwrap_or_else(|poisoned| poisoned.into_inner());

        let current_index = filters.len() - 1;
        let current_capacity =
            self.initial_capacity * self.growth_factor.powi(current_index as i32) as usize;

        if filters[current_index].estimated_items() >= current_capacity {
            let new_capacity = (current_capacity as f64 * self.growth_factor) as usize;
            let new_fp_rate = self.target_false_positive_rate
                * self.tightening_ratio.powi((current_index + 1) as i32);

            filters.push(crate::BloomFilter::new(new_capacity, new_fp_rate));
        }

        filters.last().unwrap().insert(item);
    }

    pub fn contains<T: std::hash::Hash>(&self, item: &T) -> bool {
        let filters = self
            .filters
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        filters.iter().any(|filter| filter.contains(item))
    }

    pub fn clear(&self) {
        let mut filters = self
            .filters
            .write()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        filters.clear();
        filters.push(crate::BloomFilter::new(
            self.initial_capacity,
            self.target_false_positive_rate,
        ));
    }

    pub fn filter_count(&self) -> usize {
        match self.filters.read() {
            Ok(guard) => guard.len(),
            Err(poisoned) => poisoned.into_inner().len(),
        }
    }

    pub fn estimated_items(&self) -> usize {
        let filters = self
            .filters
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        filters.iter().map(|f| f.estimated_items()).sum()
    }

    pub fn estimated_false_positive_rate(&self) -> f64 {
        let filters = self
            .filters
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());

        let mut combined_rate = 1.0;
        for filter in filters.iter() {
            let filter_fp_rate = filter.estimated_false_positive_rate();
            combined_rate *= 1.0 - filter_fp_rate;
        }

        1.0 - combined_rate
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_operations() {
        let filter = ScalableBloomFilter::new(100, 0.01, None, None);

        filter.insert(&"hello");
        filter.insert(&"world");

        assert!(filter.contains(&"hello"));
        assert!(filter.contains(&"world"));
        assert!(!filter.contains(&"foo"));
    }

    #[test]
    fn test_scaling() {
        let filter = ScalableBloomFilter::new(100, 0.01, None, None);

        for i in 0..1000 {
            filter.insert(&i);
        }

        assert!(filter.filter_count() > 1);

        for i in 0..1000 {
            assert!(filter.contains(&i));
        }
    }

    #[test]
    fn test_false_positive_rate_maintained() {
        let filter = ScalableBloomFilter::new(1000, 0.01, None, None);

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
        assert!(actual_rate < 0.03);
    }

    #[test]
    fn test_clear() {
        let filter = ScalableBloomFilter::new(100, 0.01, None, None);

        for i in 0..500 {
            filter.insert(&i);
        }

        assert!(filter.filter_count() > 1);

        filter.clear();

        assert_eq!(filter.filter_count(), 1);
        for i in 0..500 {
            assert!(!filter.contains(&i));
        }
    }

    #[test]
    fn test_concurrent_operations() {
        let filter = std::sync::Arc::new(ScalableBloomFilter::new(1000, 0.01, None, None));
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
            handle.join().expect("Thread panicked");
        }

        for i in 0..10000 {
            assert!(filter.contains(&i));
        }

        assert!(filter.filter_count() > 1);
    }

    #[test]
    fn test_estimated_items() {
        let filter = ScalableBloomFilter::new(100, 0.01, None, None);

        assert_eq!(filter.estimated_items(), 0);

        for i in 0..50 {
            filter.insert(&i);
        }
        assert_eq!(filter.estimated_items(), 50);

        for i in 50..200 {
            filter.insert(&i);
        }
        assert_eq!(filter.estimated_items(), 200);
    }

    #[test]
    fn test_estimated_false_positive_rate_api() {
        let filter = ScalableBloomFilter::new(1000, 0.01, None, None);

        for i in 0..1000 {
            filter.insert(&i);
        }

        let rate = filter.estimated_false_positive_rate();
        assert!(rate >= 0.0 && rate <= 1.0);
    }
}
