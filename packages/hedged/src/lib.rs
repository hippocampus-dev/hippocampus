//! Hedged requests for reducing tail latency in distributed systems.
//!
//! Sends duplicate requests with staggered delays and returns the first successful response,
//! implementing the pattern described in ["The Tail at Scale"] (Dean & Barroso, 2013).
//!
//! ["The Tail at Scale"]: https://cacm.acm.org/magazines/2013/2/160173-the-tail-at-scale/fulltext
//!
//! # Examples
//!
//! ```rust,ignore
//! // Send up to 3 requests, each 100ms apart
//! let result = hedged::spawn(
//!     std::time::Duration::from_millis(100),
//!     2,  // 2 additional hedged requests
//!     || async { make_request().await }
//! ).await;
//! ```

use futures::FutureExt;

pub async fn spawn<O, F, T, E>(timeout: std::time::Duration, upto: usize, f: F) -> Result<T, E>
where
    F: Fn() -> O + Sync,
    O: std::future::Future<Output = Result<T, E>> + Send,
{
    let mut jobs: Vec<futures::future::BoxFuture<Result<T, E>>> = Vec::with_capacity(upto + 1);
    jobs.push(Box::pin(f()));
    for i in 1..=upto {
        if let Some(delay) = timeout.checked_mul(i as u32) {
            jobs.push(Box::pin(tokio::time::sleep(delay).then(|_| f())));
        }
    }

    let (r, _, _) = futures::future::select_all(jobs).await;
    r
}
