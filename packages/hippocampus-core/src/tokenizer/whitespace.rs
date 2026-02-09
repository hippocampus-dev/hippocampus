#[cfg(debug_assertions)]
use elapsed::prelude::*;
use rayon::prelude::*;

#[derive(Clone, Debug, Default)]
pub struct Whitespace {}

impl Whitespace {
    pub fn new() -> Self {
        Default::default()
    }
}

impl crate::tokenizer::Tokenizer for Whitespace {
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, content)))]
    #[cfg_attr(debug_assertions, elapsed)]
    fn tokenize<S>(&mut self, content: S) -> Result<Vec<String>, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug,
    {
        Ok(content
            .as_ref()
            .par_split_whitespace()
            .map(String::from)
            .collect())
    }
}
