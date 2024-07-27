use rayon::prelude::*;

#[cfg(debug_assertions)]
use elapsed::prelude::*;

#[derive(Clone)]
pub struct Lindera {
    tokenizer: lindera::Tokenizer,
}

impl Lindera {
    pub fn new() -> Result<Self, error::Error> {
        Ok(Self {
            tokenizer: lindera::Tokenizer::from_config(lindera::TokenizerConfig {
                dictionary: lindera::DictionaryConfig {
                    kind: Some(lindera::DictionaryKind::IPADIC),
                    path: None,
                },
                user_dictionary: None,
                mode: lindera::Mode::Normal,
            })
            .map_err(|e| error::Error::from_message(e.to_string()))?,
        })
    }
}

impl crate::tokenizer::Tokenizer for Lindera {
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    fn tokenize<S>(&mut self, content: S) -> Result<Vec<String>, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug,
    {
        let tokens = content
            .as_ref()
            .par_lines()
            .map(|line| {
                Ok(self
                    .tokenizer
                    .tokenize(line)
                    .map_err(|e| error::Error::from_message(e.to_string()))?
                    .iter()
                    .map(|t| t.text.to_string())
                    .collect::<Vec<String>>())
            })
            .collect::<Result<Vec<Vec<String>>, error::Error>>()?
            .into_iter()
            .flatten()
            .collect();

        Ok(tokens)
    }
}
