use rand::Rng;

#[derive(Clone, Debug)]
pub struct NoDelay;

impl Iterator for NoDelay {
    type Item = std::time::Duration;

    fn next(&mut self) -> Option<Self::Item> {
        Some(std::time::Duration::default())
    }
}

#[derive(Clone, Debug)]
pub struct FixedDelay {
    delay: std::time::Duration,
}

impl FixedDelay {
    pub fn new(delay: std::time::Duration) -> Self {
        Self { delay }
    }
}

impl Iterator for FixedDelay {
    type Item = std::time::Duration;

    fn next(&mut self) -> Option<Self::Item> {
        Some(self.delay)
    }
}

pub trait Jitter: Fn(std::time::Duration) -> std::time::Duration + Send + Sync {
    fn clone_box(&self) -> Box<dyn Jitter>;
}

impl<T> Jitter for T
where
    T: Fn(std::time::Duration) -> std::time::Duration + Clone + Send + Sync + 'static,
{
    fn clone_box(&self) -> Box<dyn Jitter> {
        Box::new(self.clone())
    }
}

impl Clone for Box<dyn Jitter> {
    fn clone(&self) -> Self {
        self.as_ref().clone_box()
    }
}

#[derive(Clone)]
pub struct JitteredExponentialBackoff {
    next_delay: std::time::Duration,
    max_delay: Option<std::time::Duration>,
    jitter: Box<dyn Jitter>,
}

impl std::fmt::Debug for JitteredExponentialBackoff {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("JitteredExponentialBackoff")
            .field("next_delay", &self.next_delay)
            .field("max_delay", &self.max_delay)
            .finish()
    }
}

impl JitteredExponentialBackoff {
    pub fn new(base: std::time::Duration) -> Self {
        Self {
            next_delay: base,
            max_delay: None,
            jitter: Box::new(|d| rand::rng().random_range(std::time::Duration::default()..d)),
        }
    }

    pub fn set_max_delay(&mut self, max_delay: std::time::Duration) {
        self.max_delay = Some(max_delay)
    }

    pub fn set_jitter(&mut self, jitter: Box<dyn Jitter>) {
        self.jitter = jitter
    }
}

impl Iterator for JitteredExponentialBackoff {
    type Item = std::time::Duration;

    fn next(&mut self) -> Option<Self::Item> {
        let delay = if let Some(max_delay) = self.max_delay {
            std::cmp::min(self.next_delay, max_delay)
        } else {
            self.next_delay
        };

        self.next_delay = if let Some(next_delay) = self.next_delay.checked_mul(2) {
            next_delay
        } else {
            std::time::Duration::from_secs(u64::MAX)
        };

        Some((self.jitter)(delay))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn no_delay() {
        let mut s = NoDelay;
        assert_eq!(s.next(), Some(std::time::Duration::default()));
        assert_eq!(s.next(), Some(std::time::Duration::default()));
        assert_eq!(s.next(), Some(std::time::Duration::default()));
    }

    #[test]
    fn fixed_delay() {
        let mut s = FixedDelay::new(std::time::Duration::from_millis(10));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(10)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(10)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(10)));
    }

    #[test]
    fn jittered_exponential_backoff() {
        let mut s = JitteredExponentialBackoff::new(std::time::Duration::from_millis(10));
        s.set_jitter(Box::new(|d| d));
        s.set_max_delay(std::time::Duration::from_secs(1));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(10)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(20)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(40)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(80)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(160)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(320)));
        assert_eq!(s.next(), Some(std::time::Duration::from_millis(640)));
        assert_eq!(s.next(), Some(std::time::Duration::from_secs(1)));
        assert_eq!(s.next(), Some(std::time::Duration::from_secs(1)));
        assert_eq!(s.next(), Some(std::time::Duration::from_secs(1)));
    }

    #[test]
    fn overflow_jittered_exponential_backoff() {
        let mut s = JitteredExponentialBackoff::new(std::time::Duration::from_millis(1));
        s.jitter = Box::new(|d| d);
        s.set_max_delay(std::time::Duration::from_millis(1));
        for _ in 0..64 {
            assert_eq!(s.next(), Some(std::time::Duration::from_millis(1)));
        }
    }
}
