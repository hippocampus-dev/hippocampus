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
