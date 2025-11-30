const MAX_LEVEL: usize = 16;
const PROBABILITY: f64 = 0.5;

struct Node<K, V> {
    key: Option<K>,
    value: Option<V>,
    forward: Vec<Option<std::ptr::NonNull<Node<K, V>>>>,
}

impl<K, V> Node<K, V> {
    fn new(key: Option<K>, value: Option<V>, level: usize) -> Box<Self> {
        Box::new(Self {
            key,
            value,
            forward: vec![None; level + 1],
        })
    }

    #[inline]
    fn get_forward(&self, level: usize) -> Option<std::ptr::NonNull<Node<K, V>>> {
        self.forward[level]
    }

    #[inline]
    fn set_forward(&mut self, level: usize, node: Option<std::ptr::NonNull<Node<K, V>>>) {
        self.forward[level] = node;
    }
}

struct SkipListInner<K, V> {
    head: Box<Node<K, V>>,
    level: usize,
    length: usize,
}

impl<K, V> SkipListInner<K, V> {
    fn new() -> Self {
        Self {
            head: Node::new(None, None, MAX_LEVEL),
            level: 0,
            length: 0,
        }
    }

    fn len(&self) -> usize {
        self.length
    }

    fn random_level() -> usize {
        let mut level = 0;
        while level < MAX_LEVEL && fastrand::f64() < PROBABILITY {
            level += 1;
        }
        level
    }
}

impl<K: Ord + Clone, V: Clone> SkipListInner<K, V> {
    fn insert(&mut self, key: K, value: V) {
        let mut update: Vec<*mut Node<K, V>> = vec![std::ptr::null_mut(); MAX_LEVEL + 1];
        let mut current = self.head.as_mut() as *mut Node<K, V>;

        for i in (0..=self.level).rev() {
            unsafe {
                while let Some(next) = (*current).get_forward(i) {
                    let next_ptr = next.as_ptr();
                    if let Some(next_key) = (*next_ptr).key.as_ref()
                        && *next_key < key
                    {
                        current = next_ptr;
                        continue;
                    }
                    break;
                }
                update[i] = current;
            }
        }

        unsafe {
            if let Some(next) = (*current).get_forward(0) {
                let next_ptr = next.as_ptr();
                if let Some(next_key) = (*next_ptr).key.as_ref()
                    && *next_key == key
                {
                    (*next_ptr).value = Some(value);
                    return;
                }
            }
        }

        let new_level = Self::random_level();
        if new_level > self.level {
            for update_item in update.iter_mut().take(new_level + 1).skip(self.level + 1) {
                *update_item = self.head.as_mut();
            }
            self.level = new_level;
        }

        let mut new_node = Node::new(Some(key), Some(value), new_level);
        let new_node_ptr = std::ptr::NonNull::new(new_node.as_mut()).unwrap();

        for i in 0..=new_level {
            unsafe {
                new_node.set_forward(i, (*update[i]).get_forward(i));
                (*update[i]).set_forward(i, Some(new_node_ptr));
            }
        }

        std::mem::forget(new_node);
        self.length += 1;
    }

    fn get(&self, key: &K) -> Option<&V> {
        let mut current = self.head.as_ref() as *const Node<K, V>;

        for i in (0..=self.level).rev() {
            unsafe {
                while let Some(next) = (*current).get_forward(i) {
                    let next_ptr = next.as_ptr();
                    if let Some(next_key) = (*next_ptr).key.as_ref() {
                        match next_key.cmp(key) {
                            std::cmp::Ordering::Less => {
                                current = next_ptr;
                                continue;
                            }
                            std::cmp::Ordering::Equal => {
                                return (*next_ptr).value.as_ref();
                            }
                            std::cmp::Ordering::Greater => break,
                        }
                    }
                    break;
                }
            }
        }

        None
    }

    fn advance_to(&self, target: &K) -> Option<(&K, &V)> {
        let mut current = self.head.as_ref() as *const Node<K, V>;

        for i in (0..=self.level).rev() {
            unsafe {
                while let Some(next) = (*current).get_forward(i) {
                    let next_ptr = next.as_ptr();
                    if let Some(next_key) = (*next_ptr).key.as_ref()
                        && *next_key < *target
                    {
                        current = next_ptr;
                        continue;
                    }
                    break;
                }
            }
        }

        unsafe {
            if let Some(next) = (*current).get_forward(0) {
                let next_ptr = next.as_ptr();
                let key_ref = (*next_ptr).key.as_ref();
                let value_ref = (*next_ptr).value.as_ref();
                if let (Some(key), Some(value)) = (key_ref, value_ref)
                    && *key >= *target
                {
                    return Some((key, value));
                }
            }
        }

        None
    }
}

unsafe impl<K: Send, V: Send> Send for SkipListInner<K, V> {}
unsafe impl<K: Send + Sync, V: Send + Sync> Sync for SkipListInner<K, V> {}

impl<K, V> Drop for SkipListInner<K, V> {
    fn drop(&mut self) {
        let mut current = self.head.get_forward(0);
        while let Some(node) = current {
            unsafe {
                let node_ptr = node.as_ptr();
                current = (*node_ptr).get_forward(0);
                drop(Box::from_raw(node_ptr));
            }
        }
    }
}

pub struct SkipList<K, V> {
    inner: std::sync::RwLock<SkipListInner<K, V>>,
}

impl<K, V> Default for SkipList<K, V> {
    fn default() -> Self {
        Self::new()
    }
}

impl<K, V> SkipList<K, V> {
    pub fn new() -> Self {
        Self {
            inner: std::sync::RwLock::new(SkipListInner::new()),
        }
    }

    pub fn len(&self) -> usize {
        let guard = self
            .inner
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        guard.len()
    }
}

impl<K: Ord + Clone + Send + Sync, V: Clone + Send + Sync> SkipList<K, V> {
    pub fn insert(&self, key: K, value: V) {
        let mut guard = self
            .inner
            .write()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        guard.insert(key, value);
    }

    pub fn get(&self, key: &K) -> Option<V> {
        let guard = self
            .inner
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        guard.get(key).cloned()
    }

    pub fn advance_to(&self, target: &K) -> Option<(K, V)> {
        let guard = self
            .inner
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        guard.advance_to(target).map(|(k, v)| (k.clone(), v.clone()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_operations() {
        let list = SkipList::new();
        list.insert(1u64, "one".to_string());
        list.insert(2u64, "two".to_string());
        list.insert(3u64, "three".to_string());

        assert_eq!(list.len(), 3);
        assert_eq!(list.get(&1), Some("one".to_string()));
        assert_eq!(list.get(&2), Some("two".to_string()));
        assert_eq!(list.get(&3), Some("three".to_string()));
        assert_eq!(list.get(&4), None);
    }

    #[test]
    fn test_update_existing_key() {
        let list = SkipList::new();
        list.insert(1u64, "one".to_string());
        list.insert(1u64, "updated".to_string());

        assert_eq!(list.len(), 1);
        assert_eq!(list.get(&1), Some("updated".to_string()));
    }

    #[test]
    fn test_advance_to() {
        let list = SkipList::new();
        list.insert(10u64, "ten".to_string());
        list.insert(20u64, "twenty".to_string());
        list.insert(30u64, "thirty".to_string());

        assert_eq!(list.advance_to(&20), Some((20u64, "twenty".to_string())));
        assert_eq!(list.advance_to(&15), Some((20u64, "twenty".to_string())));
        assert_eq!(list.advance_to(&5), Some((10u64, "ten".to_string())));
        assert_eq!(list.advance_to(&35), None);
    }

    #[test]
    fn test_large_insert() {
        let list = SkipList::new();
        for i in 0..1000u64 {
            list.insert(i, i * 2);
        }

        assert_eq!(list.len(), 1000);

        for i in 0..1000u64 {
            assert_eq!(list.get(&i), Some(i * 2));
        }
    }

    #[test]
    fn test_concurrent_access() {
        let list = std::sync::Arc::new(SkipList::new());

        for i in 0..100u64 {
            list.insert(i, i * 2);
        }

        let mut handles = vec![];

        for _ in 0..10 {
            let list_clone = list.clone();
            let handle = std::thread::spawn(move || {
                for i in 0..100u64 {
                    assert_eq!(list_clone.get(&i), Some(i * 2));
                }
            });
            handles.push(handle);
        }

        for handle in handles {
            handle.join().unwrap();
        }
    }
}
