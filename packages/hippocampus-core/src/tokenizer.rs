pub mod lindera;
pub mod whitespace;

pub trait Tokenizer {
    fn tokenize<S>(&mut self, content: S) -> Result<Vec<String>, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug;
}
