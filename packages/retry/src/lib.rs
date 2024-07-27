pub mod strategy;

#[derive(Debug)]
pub enum Error {
    RetryExceeded(Box<dyn std::error::Error + Send + Sync + 'static>),
    Unexpected(Box<dyn std::error::Error + Send + Sync + 'static>),
}

impl std::error::Error for Error {}
impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::RetryExceeded(e) => {
                write!(f, "retry exceeded: {}", e)
            }
            Error::Unexpected(e) => {
                write!(f, "unexpected: {}", e)
            }
        }
    }
}

impl Error {
    pub fn is<E>(&self) -> bool
    where
        E: std::error::Error + Send + Sync + 'static,
    {
        match self {
            Error::RetryExceeded(e) | Error::Unexpected(e) => e.is::<E>(),
        }
    }

    pub fn downcast_ref<E>(&self) -> Option<&E>
    where
        E: std::error::Error + Send + Sync + 'static,
    {
        match self {
            Error::RetryExceeded(e) | Error::Unexpected(e) => e.downcast_ref::<E>(),
        }
    }

    pub fn downcast_mut<E>(&mut self) -> Option<&mut E>
    where
        E: std::error::Error + Send + Sync + 'static,
    {
        match self {
            Error::RetryExceeded(e) | Error::Unexpected(e) => e.downcast_mut::<E>(),
        }
    }
}

pub async fn spawn<S, O, F, T>(strategy: S, mut f: O) -> Result<T, Error>
where
    S: IntoIterator<Item = std::time::Duration>,
    O: FnMut() -> F,
    F: std::future::Future<Output = Result<T, RetryError>>,
{
    let mut strategy = strategy.into_iter();

    loop {
        match f().await {
            Ok(v) => return Ok(v),
            Err(e) => match e.0 {
                RetryErrorImpl::Retriable(e) => {
                    if let Some(delay) = strategy.next() {
                        tokio::time::sleep(delay).await;
                    } else {
                        return Err(Error::RetryExceeded(e));
                    }
                }
                RetryErrorImpl::Unexpected(e) => return Err(Error::Unexpected(e)),
            },
        }
    }
}

// Cannot convert from Box<dyn std::error::Error> https://github.com/rust-lang/rfcs/pull/2820
#[derive(Debug)]
pub struct RetryError(RetryErrorImpl);

impl std::fmt::Display for RetryError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        std::fmt::Display::fmt(&self.0, f)
    }
}

impl std::ops::Deref for RetryError {
    type Target = dyn std::error::Error + Send + Sync + 'static;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl std::ops::DerefMut for RetryError {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

impl<E> From<E> for RetryError
where
    E: std::error::Error + Send + Sync + 'static,
{
    fn from(error: E) -> Self {
        Self(RetryErrorImpl::Unexpected(Box::new(error)))
    }
}

impl RetryError {
    pub fn from_retriable_error(error: Box<dyn std::error::Error + Send + Sync + 'static>) -> Self {
        Self(RetryErrorImpl::Retriable(error))
    }

    pub fn from_unexpected_error(
        error: Box<dyn std::error::Error + Send + Sync + 'static>,
    ) -> Self {
        Self(RetryErrorImpl::Unexpected(error))
    }
}

#[derive(Debug)]
enum RetryErrorImpl {
    Retriable(Box<dyn std::error::Error + Send + Sync + 'static>),
    Unexpected(Box<dyn std::error::Error + Send + Sync + 'static>),
}

impl std::error::Error for RetryErrorImpl {}
impl std::fmt::Display for RetryErrorImpl {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            RetryErrorImpl::Retriable(e) => {
                write!(f, "retriable: {}", e)
            }
            RetryErrorImpl::Unexpected(e) => {
                write!(f, "unexpected: {}", e)
            }
        }
    }
}
