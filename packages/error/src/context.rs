pub trait ContextableError {
    fn add_context<C>(self, context: C) -> crate::Error
    where
        C: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static;
}

impl<E> ContextableError for E
where
    E: std::error::Error + Send + Sync + 'static,
{
    fn add_context<C>(self, context: C) -> crate::Error
    where
        C: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static,
    {
        crate::Error::from_context(context, self)
    }
}

impl ContextableError for crate::Error {
    fn add_context<C>(self, context: C) -> crate::Error
    where
        C: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static,
    {
        self.with_context(context)
    }
}

pub trait Context<T, E> {
    fn context<C>(self, context: C) -> Result<T, crate::Error>
    where
        C: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static;
}

impl<T, E> Context<T, E> for Result<T, E>
where
    E: ContextableError + Send + Sync + 'static,
{
    fn context<C>(self, context: C) -> Result<T, crate::Error>
    where
        C: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static,
    {
        match self {
            Ok(t) => Ok(t),
            Err(e) => Err(e.add_context(context)),
        }
    }
}
