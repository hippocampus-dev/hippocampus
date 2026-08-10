//! Draft-model speculative decoding proposer.
//!
//! A small "draft" model proposes continuation tokens that the target model
//! verifies in a single batched decode. Unlike prompt-lookup (`ngram`), which
//! only pays off when the output echoes the input, a draft model helps general
//! chat workloads -- at the cost of a second (small) model, its own context, and
//! one draft decode per proposed token.
//!
//! The proposer is proposal-agnostic to the verifier: it only produces the
//! `pending_drafts` the existing verify path already consumes. Its own KV cache
//! is reconciled against each target sequence's committed tokens by a prefix
//! match, mirroring the prompt-cache reuse in the parent module.

/// Per-processor draft model, context, and per-slot draft KV bookkeeping.
pub(crate) struct DraftModelProposer {
    model: std::sync::Arc<rust_de_llama::LlamaModel>,
    context: rust_de_llama::LlamaContext,
    sampler: rust_de_llama::LlamaSampler,
    batch_buffer: super::batch_buffer::BatchBuffer,
    n_batch: usize,
    max_draft: usize,
    n_ctx_seq: i32,
    /// Tokens currently resident in the draft context's KV, per slot sequence id.
    /// Reconciled against the target sequence's committed tokens on each proposal.
    slot_tokens: Vec<Vec<i32>>,
}

impl DraftModelProposer {
    pub fn new(
        model: std::sync::Arc<rust_de_llama::LlamaModel>,
        n_parallel: usize,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
        n_threads: i32,
        n_threads_batch: i32,
        max_draft: usize,
    ) -> Result<Self, error::Error> {
        // The draft context mirrors the target's context/sequence geometry so a
        // slot's draft positions line up with the target's. The KV stays in F16
        // and offloaded (the small draft model has a modest cache).
        let f16 = crate::config::KvCacheType::F16.to_ggml_type();
        let context = rust_de_llama::LlamaContext::new(
            &model,
            n_ctx,
            n_batch,
            n_ubatch,
            n_parallel as i32,
            n_threads,
            n_threads_batch,
            f16,
            f16,
            true,
        )
        .map_err(|e| error::error!("Failed to create draft context: {}", e))?;

        let sampler = rust_de_llama::LlamaSampler::new_greedy()
            .map_err(|e| error::error!("Failed to create draft sampler: {}", e))?;

        let n_ctx_seq = super::pad_to_kv_boundary(n_ctx);

        let mut proposer = Self {
            model,
            context,
            sampler,
            batch_buffer: super::batch_buffer::BatchBuffer::new(n_batch as usize),
            n_batch: n_batch as usize,
            max_draft,
            n_ctx_seq,
            slot_tokens: vec![Vec::new(); n_parallel],
        };
        proposer.warmup();
        Ok(proposer)
    }

    /// Fault in the draft weights and initialize its backend buffers with a
    /// single throwaway decode, so the first proposal does not pay page-in cost
    /// on top of its prefill (mirrors the target context warmup).
    fn warmup(&mut self) {
        let bos_token = unsafe { rust_de_llama::llama_vocab_bos(self.model.get_vocab()) };
        let warmup_token = if bos_token >= 0 { bos_token } else { 0 };
        self.batch_buffer.reset();
        self.batch_buffer.add_token(warmup_token, 0, 0, 1);
        if self
            .context
            .decode(self.batch_buffer.as_llama_batch())
            .is_ok()
        {
            self.context.clear_sequence(0);
        }
    }

    /// Forget a slot's draft KV (on sequence completion, disconnect, or reuse by
    /// a new task), so a reused slot never prefix-matches a previous sequence's
    /// tokens.
    pub fn reset_slot(&mut self, slot_idx: usize) {
        self.context.clear_sequence(slot_idx as i32);
        self.slot_tokens[slot_idx].clear();
    }

    /// Propose up to `max_draft` greedy continuation tokens for `slot_idx` given
    /// the target sequence's full committed token list. Syncs the draft KV to
    /// `committed` by prefix match (re-decoding only the divergent suffix), then
    /// autoregressively drafts. Returns an empty vector on any draft decode
    /// failure -- the verifier treats zero drafts as a plain decode.
    pub fn propose(&mut self, slot_idx: usize, committed: &[i32]) -> Vec<i32> {
        if self.max_draft == 0 || committed.is_empty() {
            return Vec::new();
        }

        // Keep at least one committed token to re-decode so logits for the next
        // position are produced (mirrors the parent's prompt-cache reuse cap).
        let reuse_cap = committed.len() - 1;
        let common =
            super::common_prefix_len(&self.slot_tokens[slot_idx], committed).min(reuse_cap);

        self.context
            .remove_sequence_from(slot_idx as i32, common as i32);
        self.slot_tokens[slot_idx].truncate(common);

        let suffix_start = common;
        let suffix_end = committed.len();
        let last_global = suffix_end - 1;

        // Prefill the divergent suffix in chunks bounded by n_batch (the whole
        // prompt lands here on the first proposal). Only the last token of the
        // suffix needs logits, for the first draft.
        let mut offset = suffix_start;
        let mut final_logits_index = 0i32;
        while offset < suffix_end {
            let chunk_end = (offset + self.n_batch).min(suffix_end);
            self.batch_buffer.reset();
            for (index, &token) in committed[offset..chunk_end].iter().enumerate() {
                let position = offset + index;
                let logits = if position == last_global { 1 } else { 0 };
                self.batch_buffer
                    .add_token(token, position as i32, slot_idx as i32, logits);
            }
            if self
                .context
                .decode(self.batch_buffer.as_llama_batch())
                .is_err()
            {
                self.slot_tokens[slot_idx].extend_from_slice(&committed[suffix_start..offset]);
                return Vec::new();
            }
            if chunk_end == suffix_end {
                final_logits_index = (last_global - offset) as i32;
            }
            offset = chunk_end;
        }
        self.slot_tokens[slot_idx].extend_from_slice(&committed[suffix_start..suffix_end]);

        // Autoregressively sample greedy drafts, decoding each to advance.
        let mut drafts = Vec::new();
        let mut logits_index = final_logits_index;
        let mut position = committed.len() as i32;
        for _ in 0..self.max_draft {
            if position >= self.n_ctx_seq {
                break;
            }
            let token = self.sampler.sample(&self.context, logits_index);
            if self.is_end_of_generation(token) {
                break;
            }
            drafts.push(token);

            self.batch_buffer.reset();
            self.batch_buffer
                .add_token(token, position, slot_idx as i32, 1);
            if self
                .context
                .decode(self.batch_buffer.as_llama_batch())
                .is_err()
            {
                break;
            }
            self.slot_tokens[slot_idx].push(token);
            position += 1;
            logits_index = 0;
        }

        drafts
    }

    fn is_end_of_generation(&self, token: i32) -> bool {
        let vocab = self.model.get_vocab();
        unsafe { rust_de_llama::llama_vocab_is_eog(vocab, token) }
    }
}
