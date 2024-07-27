pub mod context;

// Cannot convert from Box<dyn std::error::Error> https://github.com/rust-lang/rfcs/pull/2820
#[derive(Debug)]
pub struct Error(ErrorImpl);

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        std::fmt::Display::fmt(&self.0, f)
    }
}

impl std::ops::Deref for Error {
    type Target = dyn std::error::Error + Send + Sync + 'static;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl std::ops::DerefMut for Error {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

impl<E> From<E> for Error
where
    E: std::error::Error + Send + Sync + 'static,
{
    fn from(error: E) -> Self {
        Error::from_boxed(Box::new(error))
    }
}

impl From<Error> for Box<dyn std::error::Error + Send + Sync + 'static> {
    fn from(error: Error) -> Self {
        Box::new(error.0)
    }
}

impl From<Error> for Box<dyn std::error::Error + Send + 'static> {
    fn from(error: Error) -> Self {
        Box::<dyn std::error::Error + Send + Sync>::from(error)
    }
}

impl From<Error> for Box<dyn std::error::Error + 'static> {
    fn from(error: Error) -> Self {
        Box::<dyn std::error::Error + Send + Sync>::from(error)
    }
}

impl AsRef<dyn std::error::Error + Send + Sync> for Error {
    fn as_ref(&self) -> &(dyn std::error::Error + Send + Sync + 'static) {
        &**self
    }
}

impl AsRef<dyn std::error::Error + Send> for Error {
    fn as_ref(&self) -> &(dyn std::error::Error + Send + 'static) {
        &**self
    }
}

impl AsRef<dyn std::error::Error> for Error {
    fn as_ref(&self) -> &(dyn std::error::Error + 'static) {
        &**self
    }
}

impl Error {
    pub fn from_boxed(error: Box<dyn std::error::Error + Send + Sync>) -> Self {
        let backtrace = std::backtrace::Backtrace::capture();
        match backtrace.status() {
            std::backtrace::BacktraceStatus::Captured => Self(ErrorImpl {
                source: error,
                backtrace: Some(backtrace),
            }),
            _ => Self(ErrorImpl {
                source: error,
                backtrace: None,
            }),
        }
    }

    pub fn from_message<M>(message: M) -> Self
    where
        M: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static,
    {
        let backtrace = std::backtrace::Backtrace::capture();
        match backtrace.status() {
            std::backtrace::BacktraceStatus::Captured => Self(ErrorImpl {
                source: Box::new(MessageError(message)),
                backtrace: Some(backtrace),
            }),
            _ => Self(ErrorImpl {
                source: Box::new(MessageError(message)),
                backtrace: None,
            }),
        }
    }

    pub fn from_context<C, E>(context: C, error: E) -> Self
    where
        C: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static,
        E: std::error::Error + Send + Sync + 'static,
    {
        let backtrace = std::backtrace::Backtrace::capture();
        match backtrace.status() {
            std::backtrace::BacktraceStatus::Captured => Self(ErrorImpl {
                source: Box::new(ContextError { context, error }),
                backtrace: Some(backtrace),
            }),
            _ => Self(ErrorImpl {
                source: Box::new(ContextError { context, error }),
                backtrace: None,
            }),
        }
    }

    pub fn with_context<C>(self, context: C) -> Self
    where
        C: std::fmt::Display + std::fmt::Debug + Send + Sync + 'static,
    {
        Self(ErrorImpl {
            source: Box::new(ContextError {
                context,
                error: self,
            }),
            backtrace: None,
        })
    }

    pub fn is<E>(&self) -> bool
    where
        E: std::error::Error + Send + Sync + 'static,
    {
        self.0.source.is::<E>()
    }

    pub fn downcast_ref<E>(&self) -> Option<&E>
    where
        E: std::error::Error + Send + Sync + 'static,
    {
        self.0.source.downcast_ref::<E>()
    }

    pub fn downcast_mut<E>(&mut self) -> Option<&mut E>
    where
        E: std::error::Error + Send + Sync + 'static,
    {
        self.0.source.downcast_mut::<E>()
    }
}

#[derive(Debug)]
struct ErrorImpl {
    source: Box<dyn std::error::Error + Send + Sync + 'static>,
    backtrace: Option<std::backtrace::Backtrace>,
}

impl std::error::Error for ErrorImpl {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        Some(&*self.source)
    }
}
impl std::fmt::Display for ErrorImpl {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if let Some(backtrace) = &self.backtrace {
            write!(f, "{}\n{}", self.source, backtrace)
        } else {
            write!(f, "{}", self.source)
        }
    }
}

#[derive(Debug)]
struct MessageError<M>(M);

impl<M> std::error::Error for MessageError<M> where M: std::fmt::Display + std::fmt::Debug {}
impl<M> std::fmt::Display for MessageError<M>
where
    M: std::fmt::Display,
{
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        std::fmt::Display::fmt(&self.0, f)
    }
}

#[derive(Debug)]
struct ContextError<C, E> {
    context: C,
    error: E,
}

impl<C, E> std::error::Error for ContextError<C, E>
where
    C: std::fmt::Display + std::fmt::Debug,
    E: std::fmt::Display + std::fmt::Debug,
{
}
impl<C, E> std::fmt::Display for ContextError<C, E>
where
    C: std::fmt::Display,
    E: std::fmt::Display,
{
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}: {}", &self.context, &self.error)
    }
}

#[macro_export]
macro_rules! bail {
    ($($arg:tt)*) => {
        return Err($crate::error!($($arg)*))
    };
}

#[macro_export]
macro_rules! error {
    () => {
        $crate::format!("")
    };
    ($message:literal $(,)?) => {
        $crate::format!($message)
    };
    ($f:expr, $($arg:tt)*) => {
        $crate::format!($f, $($arg)*)
    };
}

#[macro_export]
macro_rules! format {
    ($($arg:tt)*) => {
        $crate::Error::from_message(format!($($arg)*))
    };
}
