pub struct Tokenizer {
    buffer: Vec<i32>,
}

impl Tokenizer {
    pub fn new(max_tokens: usize) -> Self {
        Self {
            buffer: vec![0i32; max_tokens],
        }
    }

    pub fn tokenize(
        &mut self,
        vocabulary: *const rust_de_llama::llama_vocab,
        text: &str,
    ) -> Result<Vec<i32>, error::Error> {
        if vocabulary.is_null() {
            return Err(error::error!("Vocabulary is null"));
        }

        let text_c = std::ffi::CString::new(text)?;
        let text_len = std::cmp::min(text.len(), self.buffer.len()) as i32;
        let token_buffer_len = self.buffer.len() as i32;

        let n_tokens = unsafe {
            rust_de_llama::llama_tokenize(
                vocabulary,
                text_c.as_ptr(),
                text_len,
                self.buffer.as_mut_ptr(),
                token_buffer_len,
                true,
                false,
            )
        };

        if n_tokens < 0 {
            return Err(error::error!("Tokenization failed"));
        }

        Ok(self.buffer[..n_tokens as usize].to_vec())
    }
}

impl Default for Tokenizer {
    fn default() -> Self {
        Self::new(2048)
    }
}
