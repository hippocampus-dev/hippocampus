use rand::Rng;

pub struct NoDelay;

impl Iterator for NoDelay {
    type Item = std::time::Duration;

    fn next(&mut self) -> Option<Self::Item> {
        Some(std::time::Duration::default())
    }
}

type Jitter = Box<dyn Fn(std::time::Duration) -> std::time::Duration + Send + Sync>;

pub struct JitteredExponentialBackoff {
    next_delay: std::time::Duration,
    max_delay: Option<std::time::Duration>,
    jitter: Jitter,
}

impl JitteredExponentialBackoff {
    pub fn new(base: std::time::Duration) -> Self {
        Self {
            next_delay: base,
            max_delay: None,
            jitter: Box::new(|d| rand::thread_rng().gen_range(std::time::Duration::default()..d)),
        }
    }

    pub fn set_max_delay(&mut self, max_delay: std::time::Duration) {
        self.max_delay = Some(max_delay)
    }

    pub fn set_jitter(&mut self, jitter: Jitter) {
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
    fn jittered_exponential_backoff() {
        let mut s = JitteredExponentialBackoff::new(std::time::Duration::from_millis(10));
        s.jitter = Box::new(|d| d);
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
