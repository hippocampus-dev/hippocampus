pub use ipc_macro::estringify;

pub mod types;

pub fn estringify<T>(
    f: impl FnOnce() -> Result<T, Box<dyn std::error::Error + Send + Sync + 'static>>,
) -> Result<T, String> {
    match f() {
        Ok(v) => Ok(v),
        Err(e) => Err(e.to_string()),
    }
}

pub async fn async_estringify<T, F>(f: impl FnOnce() -> F) -> Result<T, String>
where
    F: std::future::Future<Output = Result<T, Box<dyn std::error::Error + Send + Sync + 'static>>>,
{
    match f().await {
        Ok(v) => Ok(v),
        Err(e) => Err(e.to_string()),
    }
}

#[derive(Clone, Debug)]
pub struct Definition {
    pub file: &'static str,
    pub body: &'static str,
}

inventory::collect!(Definition);
