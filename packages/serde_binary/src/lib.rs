pub mod de;
pub mod ser;

#[derive(Debug)]
pub struct Error;
impl std::error::Error for Error {}
impl std::fmt::Display for Error {
    fn fmt(&self, _f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        todo!()
    }
}

impl serde::ser::Error for Error {
    fn custom<T>(_msg: T) -> Self
    where
        T: std::fmt::Display,
    {
        todo!()
    }
}

impl serde::de::Error for Error {
    fn custom<T>(_msg: T) -> Self
    where
        T: std::fmt::Display,
    {
        todo!()
    }
}
