pub use ipc_macro::estringify;

pub mod types;

#[derive(Clone, Debug, serde::Serialize)]
#[ipc_macro::export("error.ts")]
#[serde(tag = "kind")]
pub enum CommandError {
    Cancelled,
    Failed { message: String },
}

impl From<Box<dyn std::error::Error + Send + Sync + 'static>> for CommandError {
    fn from(error: Box<dyn std::error::Error + Send + Sync + 'static>) -> Self {
        CommandError::Failed {
            message: error.to_string(),
        }
    }
}

impl From<String> for CommandError {
    fn from(message: String) -> Self {
        CommandError::Failed { message }
    }
}

pub fn estringify<T>(
    f: impl FnOnce() -> Result<T, Box<dyn std::error::Error + Send + Sync + 'static>>,
) -> Result<T, CommandError> {
    Ok(f()?)
}

pub async fn async_estringify<T, F>(f: impl FnOnce() -> F) -> Result<T, CommandError>
where
    F: std::future::Future<Output = Result<T, Box<dyn std::error::Error + Send + Sync + 'static>>>,
{
    Ok(f().await?)
}

#[derive(Clone, Debug)]
pub struct Definition {
    pub file: &'static str,
    pub body: &'static str,
}

inventory::collect!(Definition);
