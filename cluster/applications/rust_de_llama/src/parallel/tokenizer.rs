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
        self.tokenize_text(vocabulary, text, true, false)
    }

    /// Tokenize with `<|...|>` markers resolved to their control tokens instead
    /// of literal text. The talker's prompt is assembled from such markers, so
    /// only that path opts in; chat prompts must keep them literal so user text
    /// cannot inject control tokens.
    ///
    /// `add_special` belongs to the caller because the talker's prompt is
    /// tokenized in segments: only the leading one may take the vocabulary's
    /// BOS, or it would land in the middle of the prompt (llama.cpp
    /// `tools/tts/tts.cpp` likewise passes it only in `prompt_init`).
    pub fn tokenize_special(
        &mut self,
        vocabulary: *const rust_de_llama::llama_vocab,
        text: &str,
        add_special: bool,
    ) -> Result<Vec<i32>, error::Error> {
        self.tokenize_text(vocabulary, text, add_special, true)
    }

    fn tokenize_text(
        &mut self,
        vocabulary: *const rust_de_llama::llama_vocab,
        text: &str,
        add_special: bool,
        parse_special: bool,
    ) -> Result<Vec<i32>, error::Error> {
        if vocabulary.is_null() {
            return Err(error::error!("Vocabulary is null"));
        }

        let text_c = std::ffi::CString::new(text)?;
        // Pass the full byte length; never clamp to the token buffer size, which
        // would silently truncate the prompt. Overflow is reported as an error
        // below via a negative n_tokens instead.
        let text_len = text.len() as i32;
        let token_buffer_len = self.buffer.len() as i32;

        let n_tokens = unsafe {
            rust_de_llama::llama_tokenize(
                vocabulary,
                text_c.as_ptr(),
                text_len,
                self.buffer.as_mut_ptr(),
                token_buffer_len,
                add_special,
                parse_special,
            )
        };

        if n_tokens < 0 {
            // llama_tokenize returns the negated required length when the prompt
            // does not fit the token buffer; report overflow explicitly instead
            // of silently truncating.
            return Err(error::error!(
                "Prompt is too long to tokenize: requires {} tokens but the buffer holds {}",
                -n_tokens,
                token_buffer_len
            ));
        }

        Ok(self.buffer[..n_tokens as usize].to_vec())
    }
}

impl Default for Tokenizer {
    fn default() -> Self {
        Self::new(2048)
    }
}
