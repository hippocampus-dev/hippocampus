use super::task::Task;

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
}

pub(crate) struct ActiveSequence {
    pub task: Task,
    pub n_past: usize,
    pub cache_tokens: std::collections::VecDeque<i32>,
    pub generated_tokens: Vec<i32>,
    pub prompt_token_count: usize,
    pub stop_matcher: StopMatcher,
}

pub(crate) struct Slot {
    sampler: rust_de_llama::LlamaSampler,
    pub sequence: Option<ActiveSequence>,
}

impl Slot {
    pub fn new(sampler: rust_de_llama::LlamaSampler) -> Self {
        Self {
            sampler,
            sequence: None,
        }
    }

    pub fn is_idle(&self) -> bool {
        self.sequence.is_none()
    }

    pub fn start_task(
        &mut self,
        task: Task,
        prompt_tokens: Vec<i32>,
        stop_sequences: Vec<Vec<i32>>,
        stop_strings: Vec<String>,
    ) {
        self.sequence = Some(ActiveSequence {
            task,
            n_past: 0,
            prompt_token_count: prompt_tokens.len(),
            cache_tokens: std::collections::VecDeque::from(prompt_tokens),
            generated_tokens: Vec::new(),
            stop_matcher: StopMatcher::new(stop_sequences, stop_strings),
        });
    }

    pub fn stop_task(&mut self) {
        self.sequence = None;
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
        self.sampler.sample(context, -1)
    }
}
