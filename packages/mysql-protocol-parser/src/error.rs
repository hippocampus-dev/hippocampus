//! MySQL Protocol Parser Error Types
//!
//! ## Protocol Documentation
//! - Main Protocol Overview: <https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html>

#[derive(Debug)]
pub enum Error {
    IncompletePacket(usize),
    InvalidPacket(String),
    IoError(std::io::Error),
    Utf8Error(std::string::FromUtf8Error),
}

impl std::error::Error for Error {}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::IncompletePacket(needed) => {
                write!(f, "Incomplete packet, need {needed} more bytes")
            }
            Error::InvalidPacket(message) => write!(f, "Invalid packet: {message}"),
            Error::IoError(e) => write!(f, "IO error: {e}"),
            Error::Utf8Error(e) => write!(f, "UTF-8 error: {e}"),
        }
    }
}

impl From<std::io::Error> for Error {
    fn from(e: std::io::Error) -> Self {
        Error::IoError(e)
    }
}

impl From<std::string::FromUtf8Error> for Error {
    fn from(e: std::string::FromUtf8Error) -> Self {
        Error::Utf8Error(e)
    }
}
