mod batch_buffer;
mod detokenizer;
mod slot;
mod stop_sequence;
mod tokenizer;

use slot::{ActiveSequence, CompletionReason, Slot};

pub struct Task {
    pub id: String,
    pub request: crate::handler::chat_completions::GenerateRequest,
    pub response_tx: tokio::sync::mpsc::Sender<Result<TaskResponse, error::Error>>,
    pub stop: Option<Vec<String>>,
}

pub enum TaskResponse {
    Token(String),
    Complete {
        prompt_tokens: u32,
        completion_tokens: u32,
    },
}

const DEFAULT_MAX_TOKENS: usize = 128;
const TASK_QUEUE_MULTIPLIER: usize = 4;
const BATCH_INTERVAL_MS: u64 = 1;

struct BatchSlotInfo {
    slot_idx: usize,
    tokens: Vec<i32>,
    n_past: i32,
}

struct BatchItem {
    token: i32,
    position: i32,
    seq_id: i32,
    logits: i8,
}

pub struct ParallelProcessor {
    task_tx: tokio::sync::mpsc::Sender<Task>,
    model: std::sync::Arc<rust_de_llama::LlamaModel>,
    tokenizer: std::sync::Mutex<tokenizer::Tokenizer>,
    detokenizer: std::sync::Mutex<detokenizer::Detokenizer>,
    n_ctx: i32,
    n_batch: i32,
    n_parallel: usize,
    n_ubatch: i32,
}

impl ParallelProcessor {
    pub fn new(
        model: std::sync::Arc<rust_de_llama::LlamaModel>,
        n_parallel: usize,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
    ) -> Result<(Self, tokio::sync::mpsc::Receiver<Task>), error::Error> {
        let capacity = n_parallel * TASK_QUEUE_MULTIPLIER;
        let (task_tx, task_rx) = tokio::sync::mpsc::channel(capacity);

        tracing::info!(
            "Created bounded task channel with capacity: {} (n_parallel: {})",
            capacity,
            n_parallel
        );

        Ok((
            Self {
                task_tx,
                model,
                tokenizer: std::sync::Mutex::new(tokenizer::Tokenizer::new(n_ctx as usize)),
                detokenizer: std::sync::Mutex::new(detokenizer::Detokenizer::new()),
                n_ctx,
                n_batch,
                n_parallel,
                n_ubatch,
            },
            task_rx,
        ))
    }

    pub fn submit_task(&self, task: Task) -> Result<(), error::Error> {
        self.task_tx.try_send(task).map_err(|e| match e {
            tokio::sync::mpsc::error::TrySendError::Full(_) => {
                error::error!("Task queue is full, server is overloaded")
            }
            tokio::sync::mpsc::error::TrySendError::Closed(_) => {
                error::error!("Failed to send task: channel closed")
            }
        })
    }

    fn get_available_slot(slots: &[Slot]) -> Option<usize> {
        slots
            .iter()
            .enumerate()
            .find(|(_, slot)| slot.is_idle())
            .map(|(idx, _)| idx)
    }

    async fn process_batch(
        &self,
        slots: &mut [Slot],
        context: &mut rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
    ) -> Result<bool, error::Error> {
        let slots_to_process = Self::collect_batch_slots(slots, self.n_batch);
        if slots_to_process.is_empty() {
            return Ok(false);
        }

        let (prompt_processing_slots, token_generation_slots): (Vec<_>, Vec<_>) = slots_to_process
            .iter()
            .partition(|info| !info.tokens.is_empty());

        self.process_pending_prompts(&prompt_processing_slots, context, batch_buffer)?;
        self.process_active_generation(&token_generation_slots, slots, context, batch_buffer)
            .await?;

        Ok(true)
    }

    fn process_pending_prompts(
        &self,
        prompt_slots: &[&BatchSlotInfo],
        context: &rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
    ) -> Result<(), error::Error> {
        if !prompt_slots.is_empty() {
            self.decode_prompts(prompt_slots, context, batch_buffer)?;
        }
        Ok(())
    }

    async fn process_active_generation(
        &self,
        token_generation_slots: &[&BatchSlotInfo],
        slots: &mut [Slot],
        context: &rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
    ) -> Result<(), error::Error> {
        if !token_generation_slots.is_empty() {
            let slot_indices: Vec<usize> = token_generation_slots
                .iter()
                .map(|info| info.slot_idx)
                .collect();
            self.generate_tokens(&slot_indices, slots, context, batch_buffer)
                .await?;
        }
        Ok(())
    }

    fn enqueue_and_decode(
        context: &rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
        items: impl Iterator<Item = BatchItem>,
        error_message: &str,
    ) -> Result<(), error::Error> {
        batch_buffer.reset();
        for item in items {
            batch_buffer.add_token(item.token, item.position, item.seq_id, item.logits);
        }
        let batch = batch_buffer.as_llama_batch();
        context
            .decode(batch)
            .map_err(|e| error::error!("{}: {}", error_message, e))?;
        Ok(())
    }

    fn decode_prompts(
        &self,
        prompt_slots: &[&BatchSlotInfo],
        context: &rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
    ) -> Result<(), error::Error> {
        let items = prompt_slots.iter().flat_map(|info| {
            let seq_id = info.slot_idx as i32;
            info.tokens
                .iter()
                .enumerate()
                .map(move |(i, &token)| BatchItem {
                    token,
                    position: info.n_past + i as i32,
                    seq_id,
                    logits: if i == info.tokens.len() - 1 { 1 } else { 0 },
                })
        });
        Self::enqueue_and_decode(
            context,
            batch_buffer,
            items,
            "Failed to decode prompt tokens",
        )
    }

    async fn assign_task_to_slot(
        &self,
        slot_id: usize,
        task: Task,
        slots: &mut [Slot],
    ) -> Result<(), error::Error> {
        let tokens = match self.tokenize_prompt(&task.request.prompt).await {
            Ok(t) => t,
            Err(e) => {
                let error_msg = format!("Failed to initialize task: {e}");
                let _ = task
                    .response_tx
                    .send(Err(error::error!("{}", error_msg)))
                    .await;
                return Err(e);
            }
        };

        let temperature = task.request.temperature.unwrap_or(1.0);
        let top_k = task.request.top_k.unwrap_or(64);
        let top_p = task.request.top_p.unwrap_or(0.95);
        let frequency_penalty = task.request.frequency_penalty.unwrap_or(0.0);
        let presence_penalty = task.request.presence_penalty.unwrap_or(0.0);
        let seed = task.request.seed.unwrap_or_else(rand::random);

        let mut stop_sequences = Vec::new();
        let mut stop_strings = Vec::new();
        if let Some(stops) = &task.stop {
            for stop_string in stops {
                if !stop_string.is_empty() {
                    stop_strings.push(stop_string.clone());
                    match self.tokenize_prompt(stop_string).await {
                        Ok(stop_tokens) => {
                            if !stop_tokens.is_empty() {
                                stop_sequences.push(stop_tokens);
                            }
                        }
                        Err(e) => {
                            tracing::warn!(
                                "Failed to tokenize stop sequence '{}': {}",
                                stop_string,
                                e
                            );
                        }
                    }
                }
            }
        }

        let slot = &mut slots[slot_id];
        slot.start_task(task, tokens, stop_sequences, stop_strings);
        slot.setup_sampler(
            temperature,
            top_k,
            top_p,
            frequency_penalty,
            presence_penalty,
            seed,
        );
        Ok(())
    }

    async fn tokenize_prompt(&self, prompt: &str) -> Result<Vec<i32>, error::Error> {
        let vocab = self.model.get_vocab();
        let mut tokenizer = self
            .tokenizer
            .lock()
            .map_err(|_| error::error!("Failed to lock tokenizer"))?;
        tokenizer.tokenize(vocab, prompt)
    }

    pub async fn run_processing_loop(
        self: std::sync::Arc<Self>,
        mut task_rx: tokio::sync::mpsc::Receiver<Task>,
    ) {
        let mut context =
            match rust_de_llama::LlamaContext::new(&self.model, self.n_ctx, self.n_ubatch) {
                Ok(ctx) => ctx,
                Err(e) => {
                    opentelemetry_tracing::error!("Failed to create context: {}", e);
                    return;
                }
            };

        let mut slots = Vec::with_capacity(self.n_parallel);
        for slot_id in 0..self.n_parallel {
            let sampler = match rust_de_llama::LlamaSampler::new() {
                Ok(s) => s,
                Err(e) => {
                    opentelemetry_tracing::error!(
                        "Failed to create sampler for slot {}: {}",
                        slot_id,
                        e
                    );
                    return;
                }
            };
            slots.push(Slot::new(sampler));
        }

        let mut batch_buffer = batch_buffer::BatchBuffer::new(self.n_batch as usize);
        let mut pending_tasks: std::collections::VecDeque<Task> = std::collections::VecDeque::new();
        let batch_interval = std::time::Duration::from_millis(BATCH_INTERVAL_MS);

        tracing::info!(
            "Processing loop started with batch interval: {:?}",
            batch_interval
        );

        loop {
            let has_active_slots = slots.iter().any(|slot| !slot.is_idle());
            let has_pending_work = !pending_tasks.is_empty() || has_active_slots;

            if has_pending_work {
                tokio::select! {
                    Some(task) = task_rx.recv() => {
                        pending_tasks.push_back(task);
                    }
                    _ = tokio::time::sleep(batch_interval) => {
                    }
                }
            } else {
                match task_rx.recv().await {
                    Some(task) => {
                        pending_tasks.push_back(task);
                    }
                    None => {
                        tracing::info!("Processing loop shutting down: channel closed");
                        break;
                    }
                }
            }

            while !pending_tasks.is_empty() {
                if let Some(slot_id) = Self::get_available_slot(&slots) {
                    let task = pending_tasks.pop_front().unwrap();
                    match self.assign_task_to_slot(slot_id, task, &mut slots).await {
                        Ok(()) => {}
                        Err(e) => {
                            opentelemetry_tracing::error!("Failed to assign task to slot: {}", e);
                        }
                    }
                } else {
                    break;
                }
            }

            match self
                .process_batch(&mut slots, &mut context, &mut batch_buffer)
                .await
            {
                Ok(_) => {}
                Err(e) => {
                    opentelemetry_tracing::error!("Failed to process active slots: {}", e);
                }
            }
        }
    }

    fn collect_batch_slots(slots: &mut [Slot], n_batch: i32) -> Vec<BatchSlotInfo> {
        slots
            .iter_mut()
            .enumerate()
            .filter(|(_, slot)| slot.sequence.is_some())
            .scan(0usize, |batch_size, (slot_idx, slot)| {
                let remaining_capacity = (n_batch as usize).saturating_sub(*batch_size);
                if remaining_capacity == 0 {
                    return None;
                }

                slot.next_batch_tokens(remaining_capacity)
                    .map(|(tokens, position)| {
                        let token_count = if tokens.is_empty() { 1 } else { tokens.len() };
                        *batch_size += token_count;
                        BatchSlotInfo {
                            slot_idx,
                            tokens,
                            n_past: position,
                        }
                    })
            })
            .collect()
    }

    async fn generate_tokens(
        &self,
        slot_indices: &[usize],
        slots: &mut [Slot],
        context: &rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
    ) -> Result<(), error::Error> {
        let mut results = Vec::new();
        let mut tokens_to_send = Vec::new();

        for &slot_idx in slot_indices {
            let slot = &mut slots[slot_idx];

            if slot.sequence.is_some() {
                let new_token = slot.sample_token(context);
                let is_eog = self.is_end_of_generation(new_token).await;

                {
                    let seq = slot.sequence.as_mut().unwrap();
                    seq.generated_tokens.push(new_token);
                }

                let (has_stop_sequence, stop_length) = self.check_stop_sequences(slot).await;

                let seq = slot.sequence.as_mut().unwrap();
                let max_tokens = seq
                    .task
                    .request
                    .max_tokens
                    .map(|t| t as usize)
                    .unwrap_or(DEFAULT_MAX_TOKENS);

                let completion_reason = self
                    .determine_completion_reason(
                        seq,
                        is_eog,
                        has_stop_sequence,
                        stop_length,
                        max_tokens,
                    )
                    .await;

                match completion_reason {
                    Some(CompletionReason::StopSequence(len)) => {
                        if len > 0 {
                            seq.generated_tokens
                                .truncate(seq.generated_tokens.len().saturating_sub(len));
                        }
                    }
                    Some(CompletionReason::EndOfGeneration) => {
                        seq.generated_tokens
                            .truncate(seq.generated_tokens.len().saturating_sub(1));
                    }
                    Some(CompletionReason::MaxTokens { partial_stop: true }) => {
                        // Remove partial stop sequence (e.g., "<" when stop is "</s>")
                        if let Ok(generated_text) = self.detokenize(&seq.generated_tokens).await {
                            for stop_string in seq.stop_matcher.string_patterns() {
                                if let Some(partial_pos) =
                                    stop_sequence::find_partial_stop(&generated_text, stop_string)
                                {
                                    let text_len = generated_text.len() - partial_pos;
                                    let tokens_to_remove = self
                                        .find_tokens_to_remove(
                                            &seq.generated_tokens,
                                            &generated_text,
                                            text_len,
                                        )
                                        .await;
                                    seq.generated_tokens.truncate(
                                        seq.generated_tokens.len().saturating_sub(tokens_to_remove),
                                    );
                                    break;
                                }
                            }
                        }
                    }
                    Some(CompletionReason::MaxTokens {
                        partial_stop: false,
                    }) => {}
                    None => {
                        if let Ok(text) = self.detokenize(&[new_token]).await {
                            tokens_to_send.push((seq.task.response_tx.clone(), text));
                        }
                    }
                }

                let is_complete = completion_reason.is_some();

                results.push((slot_idx, new_token, is_complete, seq.n_past as i32));
            }
        }

        for (response_tx, text) in tokens_to_send {
            let _ = response_tx.send(Ok(TaskResponse::Token(text))).await;
        }

        let tokens_to_decode: Vec<_> = results
            .iter()
            .filter(|(_, _, is_complete, _)| !is_complete)
            .map(|(idx, token, _, pos)| (*idx, *token, *pos))
            .collect();

        if !tokens_to_decode.is_empty() {
            self.decode_batch(&tokens_to_decode, context, batch_buffer, slots)?;
        }

        for (slot_idx, _, is_complete, _) in &results {
            if *is_complete {
                let slot = &mut slots[*slot_idx];
                if let Some(seq) = &slot.sequence {
                    let _ = seq
                        .task
                        .response_tx
                        .send(Ok(TaskResponse::Complete {
                            prompt_tokens: seq.prompt_token_count as u32,
                            completion_tokens: seq.generated_tokens.len() as u32,
                        }))
                        .await;
                }
                slot.stop_task();
            }
        }

        Ok(())
    }

    async fn check_stop_sequences(&self, slot: &mut Slot) -> (bool, usize) {
        let Some(seq) = &mut slot.sequence else {
            return (false, 0);
        };

        if seq.generated_tokens.is_empty() {
            return (false, 0);
        }

        // Check token-level stop sequences first
        if let Some(len) = seq.stop_matcher.check_token_stop(&seq.generated_tokens) {
            return (true, len);
        }

        // Check string-level stop sequences (llama.cpp algorithm)
        if let Ok(generated_text) = self.detokenize(&seq.generated_tokens).await {
            for stop_string in seq.stop_matcher.string_patterns() {
                if let Some(partial_pos) =
                    stop_sequence::find_partial_stop(&generated_text, stop_string)
                {
                    let text_len = generated_text.len() - partial_pos;
                    let tokens_to_remove = self
                        .find_tokens_to_remove(&seq.generated_tokens, &generated_text, text_len)
                        .await;
                    return (true, tokens_to_remove);
                }
            }
        }

        (false, 0)
    }

    /// Find how many tokens to remove to eliminate the last `text_suffix_len` characters.
    /// Uses binary search for efficiency.
    async fn find_tokens_to_remove(
        &self,
        generated_tokens: &[i32],
        generated_text: &str,
        text_suffix_len: usize,
    ) -> usize {
        if text_suffix_len >= generated_text.len() {
            return generated_tokens.len();
        }

        let target_len = generated_text.len() - text_suffix_len;

        // Binary search to find the minimal number of tokens to remove
        let mut left = 0;
        let mut right = generated_tokens.len();
        let mut result = generated_tokens.len();

        while left < right {
            let mid = (left + right) / 2;
            let tokens_to_keep = generated_tokens.len() - mid;

            if let Ok(text) = self.detokenize(&generated_tokens[..tokens_to_keep]).await {
                if text.len() <= target_len {
                    // This many tokens gives us text that's short enough
                    result = mid;
                    right = mid;
                } else {
                    // Need to remove more tokens
                    left = mid + 1;
                }
            } else {
                left = mid + 1;
            }
        }

        result
    }

    async fn is_end_of_generation(&self, token: i32) -> bool {
        let vocab = self.model.get_vocab();
        unsafe { rust_de_llama::llama_vocab_is_eog(vocab, token) }
    }

    async fn detokenize(&self, tokens: &[i32]) -> Result<String, error::Error> {
        let vocab = self.model.get_vocab();
        let mut detokenizer = self
            .detokenizer
            .lock()
            .map_err(|_| error::error!("Failed to lock detokenizer"))?;
        detokenizer.detokenize_tokens(vocab, tokens)
    }

    async fn determine_completion_reason(
        &self,
        seq: &ActiveSequence,
        is_eog: bool,
        has_stop_sequence: bool,
        stop_length: usize,
        max_tokens: usize,
    ) -> Option<CompletionReason> {
        if seq.n_past as i32 >= self.n_ctx - 1 {
            return Some(CompletionReason::MaxTokens {
                partial_stop: false,
            });
        }

        if is_eog {
            return Some(CompletionReason::EndOfGeneration);
        }

        if has_stop_sequence {
            return Some(CompletionReason::StopSequence(stop_length));
        }

        if seq.generated_tokens.len() >= max_tokens {
            // Check if we have a partial stop sequence at the end (e.g., "<" when stop is "</s>")
            let partial_stop =
                if let Ok(generated_text) = self.detokenize(&seq.generated_tokens).await {
                    seq.stop_matcher
                        .string_patterns()
                        .iter()
                        .any(|stop_string| {
                            stop_sequence::find_partial_stop(&generated_text, stop_string).is_some()
                        })
                } else {
                    false
                };
            return Some(CompletionReason::MaxTokens { partial_stop });
        }

        None
    }

    fn decode_batch(
        &self,
        token_infos: &[(usize, i32, i32)],
        context: &rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
        slots: &mut [Slot],
    ) -> Result<(), error::Error> {
        let items = token_infos
            .iter()
            .map(|(slot_idx, token, n_past)| BatchItem {
                token: *token,
                position: *n_past,
                seq_id: *slot_idx as i32,
                logits: 1,
            });

        Self::enqueue_and_decode(context, batch_buffer, items, "Batch decode failed")?;

        for (slot_idx, _, _) in token_infos.iter() {
            let slot = &mut slots[*slot_idx];
            if let Some(seq) = &mut slot.sequence {
                seq.n_past += 1;
            }
        }

        Ok(())
    }
}
