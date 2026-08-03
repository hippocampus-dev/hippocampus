use super::Task;

#[derive(Clone)]
pub(crate) enum CompletionReason {
    EndOfGeneration,
    StopSequence(usize),
    MaxTokens { partial_stop: bool },
}

pub(crate) struct StopMatcher {
    token_sequences: Vec<Vec<i32>>,
    string_patterns: Vec<String>,
}

impl StopMatcher {
    pub fn new(token_sequences: Vec<Vec<i32>>, string_patterns: Vec<String>) -> Self {
        Self {
            token_sequences,
            string_patterns,
        }
    }

    pub fn check_token_stop(&self, generated_tokens: &[i32]) -> Option<usize> {
        for stop_seq in &self.token_sequences {
            if stop_seq.len() <= generated_tokens.len() {
                let start = generated_tokens.len() - stop_seq.len();
                if generated_tokens[start..] == stop_seq[..] {
                    return Some(stop_seq.len());
                }
            }
        }
        None
    }

    pub fn string_patterns(&self) -> &[String] {
        &self.string_patterns
    }

    pub fn max_string_pattern_bytes(&self) -> usize {
        self.string_patterns
            .iter()
            .map(|pattern| pattern.len())
            .max()
            .unwrap_or(0)
    }
}

pub(crate) struct ActiveSequence {
    pub task: Task,
    pub n_past: usize,
    pub cache_tokens: std::collections::VecDeque<i32>,
    pub generated_tokens: Vec<i32>,
    pub generated_text: String,
    /// Bytes of `generated_text` already streamed to the client. The tail beyond
    /// it is withheld while it overlaps a partial stop match (stop-sequence
    /// holdback); mirrors llama.cpp server's `n_sent_text`.
    pub sent_text_bytes: usize,
    pub piece_byte_lengths: Vec<usize>,
    pub prompt_token_count: usize,
    pub stop_matcher: StopMatcher,
    pub logits_index: i32,
    /// Tokens actually decoded into this sequence's KV cells, in order. Only
    /// populated when prompt caching is enabled; on clean completion it becomes
    /// the slot's retained `cached_tokens` for prefix matching.
    pub kv_tokens: Vec<i32>,
    /// Prompt-lookup index over prompt + generated tokens, present only when
    /// prompt-lookup speculative decoding is enabled for the model.
    pub ngram: Option<super::ngram::NgramLookup>,
    /// Full committed token history (prompt + committed generated tokens), kept
    /// only for draft-model speculation to sync the draft context by prefix
    /// match. Empty when draft-model speculation is not active.
    pub spec_tokens: Vec<i32>,
    /// Draft tokens decoded in the previous batch awaiting verification this
    /// iteration (speculative decoding). Empty when there is no pending draft.
    pub pending_drafts: Vec<i32>,
}

pub(crate) struct Slot {
    sampler: rust_de_llama::LlamaSampler,
    pub sequence: Option<ActiveSequence>,
    /// Tokens whose KV cells are still resident in this slot's sequence after a
    /// clean completion (prompt caching). Empty while a sequence is active or
    /// when prompt caching is disabled.
    cached_tokens: Vec<i32>,
    /// Iterations batched since `cached_tokens` was retained.
    decode_steps_while_retained: usize,
}

impl Slot {
    pub fn new(sampler: rust_de_llama::LlamaSampler) -> Self {
        Self {
            sampler,
            sequence: None,
            cached_tokens: Vec::new(),
            decode_steps_while_retained: 0,
        }
    }

    pub fn is_idle(&self) -> bool {
        self.sequence.is_none()
    }

    /// Tokens whose KV cells this idle slot still holds (prompt caching).
    pub fn cached_tokens(&self) -> &[i32] {
        &self.cached_tokens
    }

    pub fn charge_decode_step(&mut self) {
        if !self.cached_tokens.is_empty() {
            self.decode_steps_while_retained += 1;
        }
    }

    pub fn decode_steps_while_retained(&self) -> usize {
        self.decode_steps_while_retained
    }

    pub fn start_task(
        &mut self,
        task: Task,
        prompt_tokens: Vec<i32>,
        common_len: usize,
        stop_sequences: Vec<Vec<i32>>,
        stop_strings: Vec<String>,
        speculation: Option<(usize, usize)>,
        use_draft_model: bool,
    ) {
        // `common_len` prompt tokens are already resident in KV (prompt-cache
        // reuse); only the divergent suffix needs prefilling.
        let prompt_token_count = prompt_tokens.len();
        let kv_tokens = prompt_tokens[..common_len].to_vec();
        // Draft-model speculation replaces the n-gram proposer, tracking the full
        // committed token history instead; seed it with the whole prompt. The
        // n-gram index is otherwise seeded with the whole prompt so prompt-lookup
        // can draft repeated spans from the input on the first generated token.
        let (ngram, spec_tokens) = if use_draft_model {
            (None, prompt_tokens.clone())
        } else {
            let ngram = speculation.map(|(ngram_size, max_draft)| {
                let mut lookup = super::ngram::NgramLookup::new(ngram_size, max_draft);
                lookup.extend(&prompt_tokens);
                lookup
            });
            (ngram, Vec::new())
        };
        let cache_tokens = std::collections::VecDeque::from(prompt_tokens[common_len..].to_vec());
        self.sequence = Some(ActiveSequence {
            task,
            n_past: common_len,
            prompt_token_count,
            cache_tokens,
            generated_tokens: Vec::new(),
            generated_text: String::new(),
            sent_text_bytes: 0,
            piece_byte_lengths: Vec::new(),
            stop_matcher: StopMatcher::new(stop_sequences, stop_strings),
            logits_index: -1,
            kv_tokens,
            ngram,
            spec_tokens,
            pending_drafts: Vec::new(),
        });
        // The retained tokens are now owned by the active sequence's KV.
        self.cached_tokens = Vec::new();
        self.decode_steps_while_retained = 0;
    }

    pub fn stop_task(&mut self) {
        self.sequence = None;
        self.cached_tokens = Vec::new();
        self.decode_steps_while_retained = 0;
    }

    /// Retain the completed sequence's KV cells and their token list for
    /// prefix-matched reuse instead of clearing (prompt caching). The caller
    /// must not clear the sequence's KV.
    pub fn retain_completed(&mut self) {
        if let Some(sequence) = self.sequence.take() {
            self.cached_tokens = sequence.kv_tokens;
            self.decode_steps_while_retained = 0;
        }
    }

    pub fn next_batch_tokens(&mut self, max_tokens: usize) -> Option<(Vec<i32>, i32)> {
        let seq = self.sequence.as_mut()?;

        let position = seq.n_past as i32;
        if !seq.cache_tokens.is_empty() {
            let take_count = std::cmp::min(seq.cache_tokens.len(), max_tokens);
            let tokens: Vec<i32> = seq.cache_tokens.drain(..take_count).collect();
            seq.n_past += tokens.len();
            Some((tokens, position))
        } else {
            Some((vec![], position))
        }
    }

    pub fn setup_sampler(
        &mut self,
        temperature: f32,
        top_k: i32,
        top_p: f32,
        frequency_penalty: f32,
        presence_penalty: f32,
        seed: u32,
    ) {
        self.sampler.reset();

        if frequency_penalty != 0.0 || presence_penalty != 0.0 {
            let penalties_sampler = rust_de_llama::llama_sampler_init_penalties(
                64,
                1.0,
                frequency_penalty,
                presence_penalty,
            );
            if !penalties_sampler.is_null() {
                unsafe {
                    self.sampler.chain_add(penalties_sampler);
                }
            }
        }

        let temp_sampler = rust_de_llama::llama_sampler_init_temp(temperature);
        if !temp_sampler.is_null() {
            unsafe {
                self.sampler.chain_add(temp_sampler);
            }
        }

        let top_k_sampler = rust_de_llama::llama_sampler_init_top_k(top_k);
        if !top_k_sampler.is_null() {
            unsafe {
                self.sampler.chain_add(top_k_sampler);
            }
        }

        let top_p_sampler = rust_de_llama::llama_sampler_init_top_p(top_p, 1);
        if !top_p_sampler.is_null() {
            unsafe {
                self.sampler.chain_add(top_p_sampler);
            }
        }

        let dist_sampler = rust_de_llama::llama_sampler_init_dist(seed);
        if !dist_sampler.is_null() {
            unsafe {
                self.sampler.chain_add(dist_sampler);
            }
        }
    }

    pub fn sample_token(&self, context: &rust_de_llama::LlamaContext) -> i32 {
        let logits_index = self
            .sequence
            .as_ref()
            .map(|sequence| sequence.logits_index)
            .unwrap_or(-1);
        self.sampler.sample(context, logits_index)
    }

    /// Sample from an explicit logits row, used by speculative verification to
    /// read the base row and each accepted draft's row.
    pub fn sample_token_at(&self, context: &rust_de_llama::LlamaContext, logits_index: i32) -> i32 {
        self.sampler.sample(context, logits_index)
    }
}
