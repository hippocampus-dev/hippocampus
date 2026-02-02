pub struct BatchBuffer {
    tokens: Vec<i32>,
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
        self.positions.clear();
        self.logits.clear();
        self.seq_ids.clear();
    }

    pub fn add_token(&mut self, token: i32, position: i32, sequence_id: i32, logit: i8) {
        self.tokens.push(token);
        self.positions.push(position);
        self.logits.push(logit);
        self.seq_ids.push(sequence_id);
    }

    pub fn as_llama_batch(&mut self) -> rust_de_llama::llama_batch {
        let n_tokens = self.tokens.len();

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
            token: self.tokens.as_mut_ptr(),
            embd: std::ptr::null_mut(),
            pos: self.positions.as_mut_ptr(),
            n_seq_id: self.n_seq_ids.as_mut_ptr(),
            seq_id: self.seq_id_ptrs.as_ptr() as *mut *mut i32,
            logits: self.logits.as_mut_ptr(),
        }
    }
}
