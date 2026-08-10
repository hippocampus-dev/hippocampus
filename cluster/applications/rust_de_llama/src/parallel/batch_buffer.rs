pub struct BatchBuffer {
    tokens: Vec<i32>,
    /// Flattened `[n_entries, n_embd]` input embeddings. `llama_batch` carries
    /// either `token` or `embd`, never both, so this stays empty unless the
    /// batch is built with `add_embedding`.
    embeddings: Vec<f32>,
    /// Width of `embeddings`, learned from the first `add_embedding`. llama.cpp
    /// reads `n_tokens * n_embd` floats out of `embd`, so a short entry is a
    /// heap over-read in C; the width is checked rather than assumed.
    n_embd: Option<usize>,
    positions: Vec<i32>,
    logits: Vec<i8>,
    seq_ids: Vec<i32>,
    seq_id_storage: Vec<Vec<i32>>,
    seq_id_ptrs: Vec<*mut i32>,
    n_seq_ids: Vec<i32>,
}

unsafe impl Send for BatchBuffer {}

impl BatchBuffer {
    pub fn new(capacity: usize) -> Self {
        Self {
            tokens: Vec::with_capacity(capacity),
            embeddings: Vec::new(),
            n_embd: None,
            positions: Vec::with_capacity(capacity),
            logits: Vec::with_capacity(capacity),
            seq_ids: Vec::with_capacity(capacity),
            seq_id_storage: Vec::new(),
            seq_id_ptrs: Vec::new(),
            n_seq_ids: Vec::new(),
        }
    }

    pub fn reset(&mut self) {
        self.tokens.clear();
        self.embeddings.clear();
        self.n_embd = None;
        self.positions.clear();
        self.logits.clear();
        self.seq_ids.clear();
    }

    pub fn add_token(&mut self, token: i32, position: i32, sequence_id: i32, logit: i8) {
        assert!(
            self.embeddings.is_empty(),
            "batch mixes token and embedding inputs"
        );
        self.tokens.push(token);
        self.positions.push(position);
        self.logits.push(logit);
        self.seq_ids.push(sequence_id);
    }

    /// Add one input embedding in place of a token, for models fed a projected
    /// prefix rather than vocabulary entries. All entries in a batch must be
    /// added the same way and be equally wide.
    ///
    /// Panics rather than returning an error: the alternative is handing
    /// llama.cpp an `embd` shorter than the `n_tokens * n_embd` it reads, and a
    /// caller cannot recover from having built a batch that is unsafe to pass
    /// to C.
    pub fn add_embedding(&mut self, embedding: &[f32], position: i32, sequence_id: i32, logit: i8) {
        assert!(
            self.tokens.is_empty(),
            "batch mixes token and embedding inputs"
        );
        let n_embd = *self.n_embd.get_or_insert(embedding.len());
        assert_eq!(embedding.len(), n_embd, "batch mixes embedding widths");

        self.embeddings.extend_from_slice(embedding);
        self.positions.push(position);
        self.logits.push(logit);
        self.seq_ids.push(sequence_id);
    }

    pub fn as_llama_batch(&mut self) -> rust_de_llama::llama_batch {
        // Counted from positions: the one field both input modes push per entry.
        let n_tokens = self.positions.len();
        let is_embedding_batch = !self.embeddings.is_empty();
        // llama.cpp reads n_tokens entries out of whichever pointer is set, so
        // the buffer must be exactly that long. Checked in every profile: the
        // audio path only runs under --release, where debug_assert is absent.
        assert_eq!(
            if is_embedding_batch {
                self.embeddings.len() / self.n_embd.unwrap_or(1)
            } else {
                self.tokens.len()
            },
            n_tokens,
            "batch input length does not match its positions"
        );

        self.seq_id_storage.clear();
        self.seq_id_ptrs.clear();
        self.n_seq_ids.clear();

        for &sequence_id in &self.seq_ids {
            self.seq_id_storage.push(Vec::from([sequence_id]));
        }

        for vector in &mut self.seq_id_storage {
            self.seq_id_ptrs.push(vector.as_mut_ptr());
        }

        self.n_seq_ids.resize(n_tokens, 1);

        rust_de_llama::llama_batch {
            n_tokens: n_tokens as i32,
            token: if is_embedding_batch {
                std::ptr::null_mut()
            } else {
                self.tokens.as_mut_ptr()
            },
            embd: if is_embedding_batch {
                self.embeddings.as_mut_ptr()
            } else {
                std::ptr::null_mut()
            },
            pos: self.positions.as_mut_ptr(),
            n_seq_id: self.n_seq_ids.as_mut_ptr(),
            seq_id: self.seq_id_ptrs.as_ptr() as *mut *mut i32,
            logits: self.logits.as_mut_ptr(),
        }
    }
}
