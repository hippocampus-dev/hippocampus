pub struct Detokenizer {
    buffer: [u8; 256],
}

impl Detokenizer {
    pub fn new() -> Self {
        Self { buffer: [0u8; 256] }
    }

    pub fn detokenize_tokens(
        &mut self,
        vocabulary: *const rust_de_llama::llama_vocab,
        tokens: &[i32],
    ) -> Result<String, error::Error> {
        let mut result = String::new();
        for &token in tokens {
            let piece_length = unsafe {
                rust_de_llama::llama_token_to_piece(
                    vocabulary,
                    token,
                    self.buffer.as_mut_ptr() as *mut std::os::raw::c_char,
                    self.buffer.len() as i32,
                    0,
                    false,
                )
            };

            if piece_length > 0 {
                result.push_str(&String::from_utf8_lossy(
                    &self.buffer[..piece_length as usize],
                ));
            } else {
                return Err(error::error!("Failed to detokenize token"));
            }
        }
        Ok(result)
    }
}

impl Default for Detokenizer {
    fn default() -> Self {
        Self::new()
    }
}
