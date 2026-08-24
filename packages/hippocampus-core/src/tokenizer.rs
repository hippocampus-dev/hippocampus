pub mod lindera;
#[cfg(feature = "wasm")]
pub mod wasm;
pub mod whitespace;

pub trait Tokenizer {
    fn tokenize<S>(&self, content: S) -> Result<Vec<String>, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug;
}
