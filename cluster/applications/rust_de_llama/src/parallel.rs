mod batch_buffer;
mod detokenizer;
mod draft;
mod ngram;
mod slot;
mod stop_sequence;
mod tokenizer;

use slot::{ActiveSequence, CompletionReason, Slot};

pub struct Task {
    pub id: String,
    pub request: crate::handler::chat_completions::GenerateRequest,
    /// Prompt tokens produced once during admission and reused when the task is
    /// assigned to a slot, avoiding a second tokenization under the shared lock.
    pub prompt_tokens: Vec<i32>,
    pub response_tx: tokio::sync::mpsc::Sender<Result<TaskResponse, error::Error>>,
    pub stop: Option<Vec<String>>,
}

pub enum TaskResponse {
    Token(String),
    Complete {
        prompt_tokens: u32,
        completion_tokens: u32,
        finish_reason: &'static str,
    },
}

pub const DEFAULT_MAX_TOKENS: usize = 128;
const TASK_QUEUE_MULTIPLIER: usize = 4;
/// Extra bytes scanned beyond the longest stop string so a partial match that
/// straddles a UTF-8 boundary is never missed at the tail-window edge.
const STOP_SCAN_MARGIN: usize = 8;

struct BatchSlotInfo {
    slot_idx: usize,
    tokens: Vec<i32>,
    n_past: i32,
}

/// Fires when the processing loop's stack unwinds. Declared before `context` in
/// `run_processing_loop` so it drops *after* the context, letting the model
/// manager block a replacement load until an evicted model's `LlamaContext` (the
/// committed KV allocation) is actually released -- bounding peak residency to
/// the configured cap rather than cap + 1 during a swap.
struct TeardownSignal {
    sender: Option<tokio::sync::oneshot::Sender<()>>,
}

impl Drop for TeardownSignal {
    fn drop(&mut self) {
        if let Some(sender) = self.sender.take() {
            let _ = sender.send(());
        }
    }
}

/// Result of processing one freshly sampled token: the finish reason when the
/// sequence completes on it, and the detokenized piece to stream when it is
/// emitted (`None` when the token is held back or the sequence completes).
struct TokenOutcome {
    finish_reason: Option<&'static str>,
    piece_to_send: Option<String>,
}

/// Outcome of scanning the generated text for a stop sequence. A **full** match
/// terminates generation and truncates; a **partial** match at the text end only
/// holds back the ambiguous tail from streaming while generation continues,
/// mirroring llama.cpp server (which stops on a full match and treats a partial
/// match as a reason to withhold, not to stop).
enum StopMatch {
    None,
    /// A completed stop: `byte_pos` is where the matched stop begins in
    /// `generated_text`; `tokens_to_remove` trailing tokens cover it.
    Full {
        byte_pos: usize,
        tokens_to_remove: usize,
    },
    /// A stop prefix anchored at the text end: `byte_pos` is where it begins.
    Partial {
        byte_pos: usize,
    },
}

/// Per-model serving instruments. Prefill and decode throughput are the two
/// stages this project tunes independently (thread counts, n_parallel, n_ubatch,
/// the opt-in options), so they are counted separately; batch size and slot
/// occupancy expose how well the scheduler packs work.
struct Metrics {
    prefill_tokens: opentelemetry::metrics::Counter<u64>,
    decode_tokens: opentelemetry::metrics::Counter<u64>,
    batch_size: opentelemetry::metrics::Histogram<u64>,
    active_slots: opentelemetry::metrics::Histogram<u64>,
    /// Draft tokens actually appended to the target's combined decode and the
    /// subset the target accepted; their ratio is the acceptance rate, the only
    /// measure by which max_draft, the ngram size, and the draft-model choice can
    /// be tuned (a low rate makes speculation net-negative).
    speculation_proposed_tokens: opentelemetry::metrics::Counter<u64>,
    speculation_accepted_tokens: opentelemetry::metrics::Counter<u64>,
    attributes: [opentelemetry::KeyValue; 1],
}

impl Metrics {
    fn new(meter: &opentelemetry::metrics::Meter, model_name: &str) -> Self {
        Self {
            prefill_tokens: meter
                .u64_counter("prefill_tokens_total")
                .with_description("Total number of prompt tokens prefilled")
                .init(),
            decode_tokens: meter
                .u64_counter("decode_tokens_total")
                .with_description("Total number of tokens decoded during generation")
                .init(),
            batch_size: meter
                .u64_histogram("batch_size")
                .with_description("Combined batch token count per decode")
                .init(),
            active_slots: meter
                .u64_histogram("active_slots")
                .with_description("Active slots per scheduling iteration")
                .init(),
            speculation_proposed_tokens: meter
                .u64_counter("speculation_proposed_tokens_total")
                .with_description("Total draft tokens appended to the target's combined decode")
                .init(),
            speculation_accepted_tokens: meter
                .u64_counter("speculation_accepted_tokens_total")
                .with_description("Total draft tokens the target accepted during verification")
                .init(),
            attributes: [opentelemetry::KeyValue::new(
                "model",
                model_name.to_string(),
            )],
        }
    }
}

pub struct ParallelProcessor {
    task_tx: tokio::sync::mpsc::Sender<Task>,
    model: std::sync::Arc<rust_de_llama::LlamaModel>,
    tokenizer: std::sync::Mutex<tokenizer::Tokenizer>,
    detokenizer: std::sync::Mutex<detokenizer::Detokenizer>,
    n_ctx: i32,
    n_ctx_seq: i32,
    n_batch: i32,
    n_parallel: usize,
    n_ubatch: i32,
    n_threads: i32,
    n_threads_batch: i32,
    type_k: i32,
    type_v: i32,
    offload_kqv: bool,
    prompt_cache: bool,
    /// `(ngram, max_draft)` when speculation is enabled, else None. `ngram` is
    /// unused when `draft_model` is set (the draft model replaces prompt-lookup).
    speculation: Option<(usize, usize)>,
    /// Draft model for stage-2 speculative decoding. When set, its greedy
    /// proposals replace prompt-lookup as the source of `pending_drafts`.
    draft_model: Option<std::sync::Arc<rust_de_llama::LlamaModel>>,
    metrics: Metrics,
    /// Slots currently holding a sequence, republished each scheduling iteration
    /// for the model manager's LRU eviction: a model is an eviction candidate
    /// only while this reads zero, so eviction never interrupts in-flight work.
    active_slots: std::sync::atomic::AtomicUsize,
}

impl ParallelProcessor {
    pub fn new(
        model: std::sync::Arc<rust_de_llama::LlamaModel>,
        n_parallel: usize,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
        n_threads: i32,
        n_threads_batch: i32,
        type_k: i32,
        type_v: i32,
        offload_kqv: bool,
        prompt_cache: bool,
        speculation: Option<(usize, usize)>,
        draft_model: Option<std::sync::Arc<rust_de_llama::LlamaModel>>,
        meter: &opentelemetry::metrics::Meter,
        model_name: &str,
    ) -> Result<(Self, tokio::sync::mpsc::Receiver<Task>), error::Error> {
        // Every generation slot must fit alongside prompt chunks in one batch, so
        // pass 1 of collect_batch_slots can admit all n_parallel of them.
        if n_batch < n_parallel as i32 {
            return Err(error::error!(
                "n_batch ({}) must be >= n_parallel ({}) so every generation slot fits in one batch",
                n_batch,
                n_parallel
            ));
        }

        let capacity = n_parallel * TASK_QUEUE_MULTIPLIER;
        let (task_tx, task_rx) = tokio::sync::mpsc::channel(capacity);

        // With kv_unified = false (the llama.cpp default) the per-sequence context
        // is n_ctx / n_seq_max, mirroring llama.cpp/src/llama-context.cpp:174-181.
        let n_ctx_seq = n_ctx / n_parallel.max(1) as i32;

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
                n_ctx_seq,
                n_batch,
                n_parallel,
                n_ubatch,
                n_threads,
                n_threads_batch,
                type_k,
                type_v,
                offload_kqv,
                prompt_cache,
                speculation,
                draft_model,
                metrics: Metrics::new(meter, model_name),
                active_slots: std::sync::atomic::AtomicUsize::new(0),
            },
            task_rx,
        ))
    }

    /// Tokenize a prompt for admission control before `submit_task()`. The caller
    /// checks the length and then hands the same tokens back on the `Task`, so the
    /// prompt is tokenized exactly once per request.
    pub fn tokenize_prompt_for_admission(&self, prompt: &str) -> Result<Vec<i32>, error::Error> {
        let vocab = self.model.get_vocab();
        let mut tokenizer = self
            .tokenizer
            .lock()
            .map_err(|_| error::error!("Failed to lock tokenizer"))?;
        tokenizer.tokenize(vocab, prompt)
    }

    /// Maximum prompt tokens accepted before admission is rejected. Leaves room
    /// for generation within the per-sequence context.
    pub fn max_prompt_tokens(&self, configured: Option<i32>) -> i32 {
        configured.unwrap_or_else(|| (self.n_ctx_seq - self.n_ctx_seq / 4).max(1))
    }

    /// Per-sequence context size (n_ctx / n_seq_max under kv_unified = false).
    pub fn n_ctx_seq(&self) -> i32 {
        self.n_ctx_seq
    }

    /// Slots currently generating; zero means the processing loop is parked on
    /// its idle recv and the model can be evicted without interrupting work.
    pub fn active_slots(&self) -> usize {
        self.active_slots.load(std::sync::atomic::Ordering::Relaxed)
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
        draft: &mut Option<draft::DraftModelProposer>,
    ) -> Result<bool, error::Error> {
        for slot_idx in 0..slots.len() {
            let disconnected = slots[slot_idx]
                .sequence
                .as_ref()
                .is_some_and(|seq| seq.task.response_tx.is_closed());
            if disconnected {
                slots[slot_idx].stop_task();
                context.clear_sequence(slot_idx as i32);
                if let Some(draft) = draft.as_mut() {
                    draft.reset_slot(slot_idx);
                }
            }
        }

        let slots_to_process = Self::collect_batch_slots(slots, self.n_batch);
        if slots_to_process.is_empty() {
            return Ok(false);
        }

        let active_slot_count = slots.iter().filter(|slot| !slot.is_idle()).count();
        self.metrics.active_slots.record(
            &opentelemetry::Context::current(),
            active_slot_count as u64,
            &self.metrics.attributes,
        );

        let (prompt_processing_slots, token_generation_slots): (Vec<_>, Vec<_>) = slots_to_process
            .iter()
            .partition(|info| !info.tokens.is_empty());

        // Sample the active generation slots from the *previous* decode before
        // issuing this iteration's decode. llama.cpp keeps only the last decode's
        // logits buffer, so sampling must not be preceded by any other decode in
        // the same iteration -- otherwise a slot's recorded `logits_index` would
        // point into a foreign (or out-of-range) output row, yielding wrong logits
        // or aborting via GGML_ASSERT.
        let generation_slot_indices: Vec<usize> = token_generation_slots
            .iter()
            .map(|info| info.slot_idx)
            .collect();
        let continuing = self
            .sample_generation_slots(&generation_slot_indices, slots, context, draft)
            .await;

        // Prefill and generation-continuation tokens go into a single decode, so
        // every active slot gets exactly one logits = 1 output row this iteration.
        self.decode_combined(
            &prompt_processing_slots,
            &continuing,
            slots,
            context,
            batch_buffer,
        )?;

        Ok(true)
    }

    /// Build one combined batch from the prefilling slots (prompt tokens, last
    /// token marked as output) and the continuing generation slots (one sampled
    /// token each), decode it once, and record each slot's output-row index for
    /// the next iteration's sampling.
    fn decode_combined(
        &self,
        prompt_slots: &[&BatchSlotInfo],
        continuing: &[(usize, i32, i32)],
        slots: &mut [Slot],
        context: &rust_de_llama::LlamaContext,
        batch_buffer: &mut batch_buffer::BatchBuffer,
    ) -> Result<(), error::Error> {
        if prompt_slots.is_empty() && continuing.is_empty() {
            return Ok(());
        }

        batch_buffer.reset();
        // `logits_index` must be the token's position within this batch, not an
        // output-row ordinal: llama_get_logits_ith(i) treats a non-negative i as
        // a batch token index and maps it through output_ids[i] to the row
        // (llama-context.h:314). A position whose logits flag is 0 has
        // output_ids[i] == -1, which aborts sampling via GGML_ASSERT.
        let mut batch_position = 0i32;
        // Draft tokens appended below after the per-sequence budget caps; counted
        // so the acceptance rate can be derived against the accepted count.
        let mut proposed_draft_count = 0usize;

        for info in prompt_slots {
            let seq_id = info.slot_idx as i32;
            let last_index = info.tokens.len().saturating_sub(1);
            for (i, &token) in info.tokens.iter().enumerate() {
                let logits = if i == last_index { 1 } else { 0 };
                batch_buffer.add_token(token, info.n_past + i as i32, seq_id, logits);
            }
            // The only logits = 1 token is the last one of this prompt chunk.
            if let Some(sequence) = slots[info.slot_idx].sequence.as_mut() {
                sequence.logits_index = batch_position + last_index as i32;
                // Record the tokens now committed to KV so a clean completion can
                // retain them for prefix-matched reuse (prompt caching).
                if self.prompt_cache {
                    sequence.kv_tokens.extend_from_slice(&info.tokens);
                }
            }
            batch_position += info.tokens.len() as i32;
        }

        for (continuing_index, &(slot_idx, token, position)) in continuing.iter().enumerate() {
            batch_buffer.add_token(token, position, slot_idx as i32, 1);
            if let Some(sequence) = slots[slot_idx].sequence.as_mut() {
                sequence.logits_index = batch_position;
                if self.prompt_cache {
                    sequence.kv_tokens.push(token);
                }
            }
            batch_position += 1;

            // Append this slot's speculative drafts immediately after its
            // continuing token, each with logits, at the next tentative
            // positions -- bounded by the remaining batch budget so the combined
            // decode never exceeds n_batch. They are verified (and the rejected
            // tail rolled back) next iteration.
            if self.speculation.is_some() {
                let drafts = slots[slot_idx]
                    .sequence
                    .as_ref()
                    .map(|sequence| sequence.pending_drafts.clone())
                    .unwrap_or_default();
                // Bound by remaining batch budget and by remaining per-sequence
                // context, so drafts never overflow n_batch or n_ctx_seq (which
                // would abort the decode). Reserve one batch slot for each later
                // generation slot's still-unadded continuing token, which is
                // mandatory and must always fit.
                let reserved_for_later_continuing = continuing.len() - (continuing_index + 1);
                let remaining_batch =
                    (self.n_batch - batch_position - reserved_for_later_continuing as i32).max(0)
                        as usize;
                let remaining_ctx = (self.n_ctx_seq - (position + 1)).max(0) as usize;
                let take = drafts.len().min(remaining_batch).min(remaining_ctx);
                proposed_draft_count += take;
                for (offset, &draft) in drafts.iter().take(take).enumerate() {
                    batch_buffer.add_token(draft, position + 1 + offset as i32, slot_idx as i32, 1);
                    batch_position += 1;
                }
                if take < drafts.len() {
                    if let Some(sequence) = slots[slot_idx].sequence.as_mut() {
                        sequence.pending_drafts.truncate(take);
                    }
                }
            }
        }

        let batch = batch_buffer.as_llama_batch();
        context
            .decode(batch)
            .map_err(|e| error::error!("Combined decode failed: {}", e))?;

        // Prefill (prompt chunk) and decode (one-per-generation-slot) tokens are
        // counted separately so their throughput can be tuned independently.
        let prefill_count: usize = prompt_slots.iter().map(|info| info.tokens.len()).sum();
        let decode_count = continuing.len();
        let otel_context = opentelemetry::Context::current();
        if prefill_count > 0 {
            self.metrics.prefill_tokens.add(
                &otel_context,
                prefill_count as u64,
                &self.metrics.attributes,
            );
        }
        if decode_count > 0 {
            self.metrics.decode_tokens.add(
                &otel_context,
                decode_count as u64,
                &self.metrics.attributes,
            );
        }
        self.metrics.batch_size.record(
            &otel_context,
            (prefill_count + decode_count) as u64,
            &self.metrics.attributes,
        );
        if proposed_draft_count > 0 {
            self.metrics.speculation_proposed_tokens.add(
                &otel_context,
                proposed_draft_count as u64,
                &self.metrics.attributes,
            );
        }

        // Prompt slots already advanced n_past in next_batch_tokens; the
        // continuing slots consumed one more position with this decode.
        for &(slot_idx, _, _) in continuing {
            if let Some(sequence) = slots[slot_idx].sequence.as_mut() {
                sequence.n_past += 1;
            }
        }

        Ok(())
    }

    async fn assign_task_to_slot(
        &self,
        mut task: Task,
        slots: &mut [Slot],
        context: &rust_de_llama::LlamaContext,
    ) -> Result<(), error::Error> {
        // Reuse the tokens produced during admission instead of tokenizing again.
        let tokens = std::mem::take(&mut task.prompt_tokens);

        // Choose the slot: the longest shared token prefix among idle slots when
        // prompt caching is enabled, otherwise the first idle slot.
        let (slot_id, common_len) = match self.select_slot(slots, &tokens) {
            Some(selection) => selection,
            None => {
                let _ = task
                    .response_tx
                    .send(Err(error::error!("No idle slot available")))
                    .await;
                return Err(error::error!("No idle slot available"));
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

        // Drop the divergent KV tail of the chosen slot, keeping the shared
        // prefix. A no-op when prompt caching is off (cells already cleared on
        // completion) or when the whole retained sequence is reused.
        if self.prompt_cache {
            context.remove_sequence_from(slot_id as i32, common_len as i32);
        }

        let slot = &mut slots[slot_id];
        slot.start_task(
            task,
            tokens,
            common_len,
            stop_sequences,
            stop_strings,
            self.speculation,
            self.draft_model.is_some(),
        );
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

    /// Select the slot to assign a new prompt to and how many leading prompt
    /// tokens are already resident in its KV. Without prompt caching this is the
    /// first idle slot with `common_len = 0`; with it, the idle slot sharing the
    /// longest exact token prefix, capped at `prompt_tokens.len() - 1` so at
    /// least one token is always prefilled to produce a logits row.
    fn select_slot(&self, slots: &[Slot], prompt_tokens: &[i32]) -> Option<(usize, usize)> {
        if !self.prompt_cache {
            return Self::get_available_slot(slots).map(|slot_id| (slot_id, 0));
        }

        let reuse_cap = prompt_tokens.len().saturating_sub(1);
        let mut best: Option<(usize, usize)> = None;
        for (slot_id, slot) in slots.iter().enumerate() {
            if !slot.is_idle() {
                continue;
            }
            let common_len = common_prefix_len(slot.cached_tokens(), prompt_tokens).min(reuse_cap);
            if best.is_none_or(|(_, best_common)| common_len > best_common) {
                best = Some((slot_id, common_len));
            }
        }
        best
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
        mut shutdown_rx: tokio::sync::watch::Receiver<bool>,
        teardown_tx: tokio::sync::oneshot::Sender<()>,
    ) {
        // Declared before `context` so it drops last, signalling the manager only
        // once the context's KV memory is freed (used to bound peak residency on
        // eviction). Any exit path -- context/sampler init failure or the shutdown
        // break -- releases it.
        let _teardown_signal = TeardownSignal {
            sender: Some(teardown_tx),
        };

        let mut context = match rust_de_llama::LlamaContext::new(
            &self.model,
            self.n_ctx,
            self.n_batch,
            self.n_ubatch,
            self.n_parallel as i32,
            self.n_threads,
            self.n_threads_batch,
            self.type_k,
            self.type_v,
            self.offload_kqv,
        ) {
            Ok(ctx) => ctx,
            Err(e) => {
                opentelemetry_tracing::error!("Failed to create context: {}", e);
                return;
            }
        };

        // Fault in the mmap'd weights and initialize every backend buffer with a
        // single throwaway decode, so the first real request no longer pays the
        // page-in of gigabytes of weights plus backend setup on top of its own
        // prefill. llama.cpp's tools warm up at load for the same reason.
        {
            let warmup_start = std::time::Instant::now();
            let bos_token = unsafe { rust_de_llama::llama_vocab_bos(self.model.get_vocab()) };
            let warmup_token = if bos_token >= 0 { bos_token } else { 0 };
            let mut warmup_batch = batch_buffer::BatchBuffer::new(1);
            warmup_batch.add_token(warmup_token, 0, 0, 1);
            match context.decode(warmup_batch.as_llama_batch()) {
                Ok(()) => {
                    context.clear_sequence(0);
                    tracing::info!("Warmup decode completed in {:?}", warmup_start.elapsed());
                }
                Err(e) => tracing::warn!("Warmup decode failed: {}", e),
            }
        }

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

        // Stage-2 draft-model proposer, built once alongside the target context.
        // On failure the loop degrades to no drafts (a plain decode), never an
        // abort, so a bad draft configuration cannot take the model offline.
        let mut draft_proposer: Option<draft::DraftModelProposer> = match &self.draft_model {
            Some(draft_model) => {
                let max_draft = self
                    .speculation
                    .map(|(_, max_draft)| max_draft)
                    .unwrap_or(0);
                match draft::DraftModelProposer::new(
                    draft_model.clone(),
                    self.n_parallel,
                    self.n_ctx,
                    self.n_batch,
                    self.n_ubatch,
                    self.n_threads,
                    self.n_threads_batch,
                    max_draft,
                ) {
                    Ok(proposer) => {
                        tracing::info!(
                            "Draft-model speculative decoding enabled (max_draft: {})",
                            max_draft
                        );
                        Some(proposer)
                    }
                    Err(e) => {
                        opentelemetry_tracing::error!(
                            "Failed to initialize draft model, disabling draft speculation: {}",
                            e
                        );
                        None
                    }
                }
            }
            None => None,
        };

        tracing::info!("Processing loop started");

        loop {
            // Republish the busy-slot count for the model manager's eviction: it
            // evicts only while this reads zero, and this branch sets it to zero
            // exactly when the loop is about to park on its idle recv below.
            let active_slot_count = slots.iter().filter(|slot| !slot.is_idle()).count();
            self.active_slots
                .store(active_slot_count, std::sync::atomic::Ordering::Relaxed);
            let has_pending_work = !pending_tasks.is_empty() || active_slot_count > 0;

            if has_pending_work {
                // Drain everything queued so newly freed slots refill promptly
                // after a decode, then fall through to process_batch without
                // sleeping. The loop cadence is set by decode latency, not a
                // fixed interval, so this adds no busy-wait.
                loop {
                    match task_rx.try_recv() {
                        Ok(task) => pending_tasks.push_back(task),
                        Err(_) => break,
                    }
                }
            } else if *shutdown_rx.borrow() {
                // Evicted: honor the latched eviction signal, but drain any task
                // that raced into the channel buffer first so an already-admitted
                // request is never dropped mid-eviction. Only tear down once the
                // buffer is empty; drained tasks are processed like any other.
                match task_rx.try_recv() {
                    Ok(task) => pending_tasks.push_back(task),
                    Err(_) => {
                        tracing::info!("Processing loop shutting down: model evicted");
                        break;
                    }
                }
            } else {
                // Idle: all slots empty, so accept the next task or wake on an
                // eviction signal. task_rx is polled first so a buffered task is
                // always taken over the signal; the signal is latched and honored
                // by the branch above on the next iteration.
                tokio::select! {
                    biased;
                    received = task_rx.recv() => match received {
                        Some(task) => pending_tasks.push_back(task),
                        None => {
                            tracing::info!("Processing loop shutting down: channel closed");
                            break;
                        }
                    },
                    // Wake to re-check the latched signal above. An Err means every
                    // sender was dropped without an eviction signal (the manager is
                    // gone), so tear down rather than spin on a closed watch.
                    changed = shutdown_rx.changed() => {
                        if changed.is_err() {
                            break;
                        }
                    }
                }
            }

            while !pending_tasks.is_empty() {
                // The concrete slot (and any prompt-cache prefix) is chosen inside
                // assign_task_to_slot after tokenizing; here we only need to know
                // an idle slot exists before consuming the task.
                if slots.iter().any(|slot| slot.is_idle()) {
                    let task = pending_tasks.pop_front().unwrap();
                    match self.assign_task_to_slot(task, &mut slots, &context).await {
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
                .process_batch(
                    &mut slots,
                    &mut context,
                    &mut batch_buffer,
                    &mut draft_proposer,
                )
                .await
            {
                Ok(_) => {}
                Err(e) => {
                    opentelemetry_tracing::error!("Failed to process active slots: {}", e);
                }
            }
        }
    }

    /// Collect this iteration's batch contributions in two passes. Pass 1 admits
    /// every active generation slot (one token each, identified by an empty
    /// `cache_tokens`); pass 2 fills the remaining budget with prompt chunks.
    /// `n_parallel <= n_batch` is guaranteed at startup, so generation slots
    /// always fit and a long prefill can never starve an in-flight generation of
    /// its decode -- which would otherwise leave a stale `logits_index` (abort or
    /// wrong logits) and spike inter-token latency by the whole prefill duration.
    fn collect_batch_slots(slots: &mut [Slot], n_batch: i32) -> Vec<BatchSlotInfo> {
        let mut batch_size = 0usize;
        let mut collected = Vec::new();

        for (slot_idx, slot) in slots.iter_mut().enumerate() {
            let is_generation = slot
                .sequence
                .as_ref()
                .is_some_and(|sequence| sequence.cache_tokens.is_empty());
            if !is_generation {
                continue;
            }
            if let Some((tokens, position)) = slot.next_batch_tokens(n_batch as usize) {
                // A generation slot contributes exactly one token.
                batch_size += 1;
                collected.push(BatchSlotInfo {
                    slot_idx,
                    tokens,
                    n_past: position,
                });
            }
        }

        for (slot_idx, slot) in slots.iter_mut().enumerate() {
            let is_prompt = slot
                .sequence
                .as_ref()
                .is_some_and(|sequence| !sequence.cache_tokens.is_empty());
            if !is_prompt {
                continue;
            }
            let remaining_capacity = (n_batch as usize).saturating_sub(batch_size);
            if remaining_capacity == 0 {
                break;
            }
            if let Some((tokens, position)) = slot.next_batch_tokens(remaining_capacity) {
                let token_count = if tokens.is_empty() { 1 } else { tokens.len() };
                batch_size += token_count;
                collected.push(BatchSlotInfo {
                    slot_idx,
                    tokens,
                    n_past: position,
                });
            }
        }

        collected
    }

    /// Sample one token for each active generation slot from the previous
    /// decode, handle stop/completion/streaming, and return the slots that keep
    /// generating as `(slot_idx, new_token, position)` for the combined decode.
    async fn sample_generation_slots(
        &self,
        slot_indices: &[usize],
        slots: &mut [Slot],
        context: &rust_de_llama::LlamaContext,
        draft: &mut Option<draft::DraftModelProposer>,
    ) -> Vec<(usize, i32, i32)> {
        let mut results: Vec<(usize, i32, Option<&'static str>, i32)> = Vec::new();
        let mut tokens_to_send: Vec<(usize, String)> = Vec::new();

        for &slot_idx in slot_indices {
            let slot = &mut slots[slot_idx];

            // Stop generating when the client has disconnected — the response
            // channel is closed once the streaming body / non-streaming handler
            // future is dropped by axum on connection close.
            if let Some(seq) = &slot.sequence {
                if seq.task.response_tx.is_closed() {
                    slot.stop_task();
                    context.clear_sequence(slot_idx as i32);
                    if let Some(draft) = draft.as_mut() {
                        draft.reset_slot(slot_idx);
                    }
                    continue;
                }
            }

            if slot.sequence.is_none() {
                continue;
            }

            if self.speculation.is_some() {
                // Speculative path: verify last iteration's drafts, commit the
                // accepted run, and roll back the rejected KV tail.
                self.sample_generation_slot_speculative(
                    slot_idx,
                    slots,
                    context,
                    draft,
                    &mut results,
                    &mut tokens_to_send,
                )
                .await;
            } else {
                let new_token = slots[slot_idx].sample_token(context);
                let outcome = self
                    .handle_sampled_token(&mut slots[slot_idx], new_token)
                    .await;
                if let Some(text) = outcome.piece_to_send {
                    tokens_to_send.push((slot_idx, text));
                }
                let n_past = slots[slot_idx]
                    .sequence
                    .as_ref()
                    .map(|seq| seq.n_past as i32)
                    .unwrap_or(0);
                results.push((slot_idx, new_token, outcome.finish_reason, n_past));
            }
        }

        // Deliver tokens without ever blocking the scheduling loop on a single
        // slow client: with the channel sized to the token budget, Full is
        // unreachable for healthy clients, so a Full/Closed send means the
        // client is gone. Mark it disconnected and reclaim the slot below.
        let mut disconnected: std::collections::HashSet<usize> = std::collections::HashSet::new();
        for (slot_idx, text) in tokens_to_send {
            let sender = slots[slot_idx]
                .sequence
                .as_ref()
                .map(|seq| seq.task.response_tx.clone());
            if let Some(sender) = sender {
                if sender.try_send(Ok(TaskResponse::Token(text))).is_err() {
                    disconnected.insert(slot_idx);
                }
            }
        }

        for (slot_idx, _, finish_reason, _) in &results {
            if let &Some(reason) = finish_reason {
                if let Some(seq) = &slots[*slot_idx].sequence {
                    let _ = seq.task.response_tx.try_send(Ok(TaskResponse::Complete {
                        prompt_tokens: seq.prompt_token_count as u32,
                        completion_tokens: seq.generated_tokens.len() as u32,
                        finish_reason: reason,
                    }));
                }
                if self.prompt_cache {
                    // Keep the KV cells and their token list for prefix-matched
                    // reuse by a later request; clearing is skipped on purpose.
                    slots[*slot_idx].retain_completed();
                } else {
                    slots[*slot_idx].stop_task();
                    context.clear_sequence(*slot_idx as i32);
                }
                // The draft context does not participate in prompt-cache reuse, so
                // its cells for this slot are always dropped on completion.
                if let Some(draft) = draft.as_mut() {
                    draft.reset_slot(*slot_idx);
                }
            }
        }

        for &slot_idx in &disconnected {
            slots[slot_idx].stop_task();
            context.clear_sequence(slot_idx as i32);
            if let Some(draft) = draft.as_mut() {
                draft.reset_slot(slot_idx);
            }
        }

        // Slots that neither completed nor disconnected continue generating; their
        // sampled token is decoded in the combined batch at `position` (n_past).
        results
            .into_iter()
            .filter(|(slot_idx, _, finish_reason, _)| {
                finish_reason.is_none() && !disconnected.contains(slot_idx)
            })
            .map(|(slot_idx, token, _, position)| (slot_idx, token, position))
            .collect()
    }

    /// Process one freshly sampled token for a slot: grow the incremental
    /// generated buffers, evaluate stop/EOG/max-token completion, apply the
    /// matching truncation, and report the finish reason plus the piece to stream
    /// when the token is emitted.
    async fn handle_sampled_token(&self, slot: &mut Slot, new_token: i32) -> TokenOutcome {
        let is_eog = self.is_end_of_generation(new_token).await;

        // Detokenize only this token and grow the incremental text buffer; the
        // piece is reused for the streaming send and keeps `piece_byte_lengths`
        // aligned 1:1 with `generated_tokens`.
        let piece = self.detokenize(&[new_token]).await.ok();
        {
            let seq = slot.sequence.as_mut().unwrap();
            seq.generated_tokens.push(new_token);
            match &piece {
                Some(text) => {
                    seq.generated_text.push_str(text);
                    seq.piece_byte_lengths.push(text.len());
                }
                None => seq.piece_byte_lengths.push(0),
            }
        }

        let stop_match = self.check_stop_sequences(slot).await;

        let seq = slot.sequence.as_mut().unwrap();
        let max_tokens = seq
            .task
            .request
            .max_tokens
            .map(|t| t as usize)
            .unwrap_or(DEFAULT_MAX_TOKENS);

        let completion_reason = self
            .determine_completion_reason(seq, is_eog, &stop_match, max_tokens)
            .await;

        let text_len = seq.generated_text.len();
        let last_piece_len = seq.piece_byte_lengths.last().copied().unwrap_or(0);

        // Streaming send-boundary: the byte offset of `generated_text` up to which
        // it may be flushed to the client on this token. Bytes in
        // `[sent_text_bytes, send_boundary)` are streamed; bytes beyond it are
        // either withheld (a pending partial stop) or discarded (a matched stop,
        // or the token that tripped a completion boundary).
        let send_boundary: usize = match &completion_reason {
            Some(CompletionReason::StopSequence(tokens_to_remove)) => {
                // Full stop: drop the matched tail from the token count, flush the
                // text before the stop, and discard the stop plus any held tail.
                if *tokens_to_remove > 0 {
                    seq.generated_tokens
                        .truncate(seq.generated_tokens.len().saturating_sub(*tokens_to_remove));
                }
                match &stop_match {
                    StopMatch::Full { byte_pos, .. } => *byte_pos,
                    // Unreachable: StopSequence is produced only from a full match.
                    _ => text_len.saturating_sub(last_piece_len),
                }
            }
            Some(CompletionReason::EndOfGeneration) => {
                // Drop the EOG token; flush everything before it, including any
                // held partial tail -- the partial never completed, so it is real
                // output.
                seq.generated_tokens
                    .truncate(seq.generated_tokens.len().saturating_sub(1));
                text_len.saturating_sub(last_piece_len)
            }
            Some(CompletionReason::MaxTokens { partial_stop: true }) => {
                // Token budget reached with a partial stop at the end: trim it from
                // the token count and withhold it from the stream, as it may have
                // completed into a stop had generation continued.
                match &stop_match {
                    StopMatch::Partial { byte_pos } => {
                        let suffix_len = text_len - *byte_pos;
                        let tokens_to_remove =
                            count_tokens_for_suffix(&seq.piece_byte_lengths, suffix_len);
                        seq.generated_tokens
                            .truncate(seq.generated_tokens.len().saturating_sub(tokens_to_remove));
                        *byte_pos
                    }
                    _ => text_len,
                }
            }
            Some(CompletionReason::MaxTokens {
                partial_stop: false,
            }) => {
                // Budget/context limit with no partial tail: the token that tripped
                // the boundary is not streamed (unchanged), but any earlier held
                // text is flushed.
                text_len.saturating_sub(last_piece_len)
            }
            None => {
                // Not completing: withhold a pending partial-stop tail and keep
                // generating; flush everything else.
                match &stop_match {
                    StopMatch::Partial { byte_pos } => *byte_pos,
                    _ => text_len,
                }
            }
        };

        let piece_to_send = if send_boundary > seq.sent_text_bytes {
            let flushed = seq.generated_text[seq.sent_text_bytes..send_boundary].to_string();
            seq.sent_text_bytes = send_boundary;
            Some(flushed)
        } else {
            None
        };

        // MaxTokens → "length", stop/EOG → "stop" (OpenAI spec).
        let finish_reason: Option<&'static str> =
            completion_reason.as_ref().map(|reason| match reason {
                CompletionReason::MaxTokens { .. } => "length",
                CompletionReason::StopSequence(_) | CompletionReason::EndOfGeneration => "stop",
            });

        TokenOutcome {
            finish_reason,
            piece_to_send,
        }
    }

    /// Speculative verification for one generation slot. Reads the base logits
    /// row (the continuing token decoded last iteration) and each accepted
    /// draft's row, committing the longest run the target model agrees with; the
    /// first non-matching (or bonus) token becomes the next continuing token. The
    /// rejected draft tail is rolled back so KV holds exactly `n_past` cells.
    ///
    /// Because every committed token is sampled from the correct target row with
    /// the slot's own sampler, in exactly the same order and count as the
    /// non-speculative path, the produced sequence is identical -- accepted
    /// drafts are simply free.
    async fn sample_generation_slot_speculative(
        &self,
        slot_idx: usize,
        slots: &mut [Slot],
        context: &rust_de_llama::LlamaContext,
        draft: &mut Option<draft::DraftModelProposer>,
        results: &mut Vec<(usize, i32, Option<&'static str>, i32)>,
        tokens_to_send: &mut Vec<(usize, String)>,
    ) {
        // Base logits row and the drafts decoded alongside the continuing token
        // last iteration. Draft rows follow the base row at +1, +2, ...
        let (base_index, pending) = {
            let seq = slots[slot_idx].sequence.as_ref().unwrap();
            (seq.logits_index, seq.pending_drafts.clone())
        };

        let mut matched = 0usize;
        let mut sample_index = base_index;
        // On completion, `(reason, token)`; the token value is unused for
        // completed slots but keeps the results tuple shape uniform.
        let mut finished: Option<(&'static str, i32)> = None;
        let mut continuing_token: Option<i32> = None;

        loop {
            let token = slots[slot_idx].sample_token_at(context, sample_index);
            let is_draft_match = matched < pending.len() && token == pending[matched];

            // A matched draft is already resident in KV; commit it to n_past, the
            // retained-token list, and the proposer's history before handling it.
            if is_draft_match {
                if let Some(seq) = slots[slot_idx].sequence.as_mut() {
                    seq.n_past += 1;
                    if self.prompt_cache {
                        seq.kv_tokens.push(token);
                    }
                    if let Some(lookup) = seq.ngram.as_mut() {
                        lookup.push(token);
                    }
                    if self.draft_model.is_some() {
                        seq.spec_tokens.push(token);
                    }
                }
            }

            let outcome = self.handle_sampled_token(&mut slots[slot_idx], token).await;
            if let Some(text) = outcome.piece_to_send {
                tokens_to_send.push((slot_idx, text));
            }
            if let Some(reason) = outcome.finish_reason {
                finished = Some((reason, token));
                break;
            }

            if is_draft_match {
                matched += 1;
                sample_index = base_index + matched as i32;
            } else {
                // First non-matching token (or the bonus past the last draft):
                // the new continuing token, still undecoded.
                continuing_token = Some(token);
                break;
            }
        }

        // Accepted drafts of this verify run; their ratio to the proposed count
        // recorded at the append site is the acceptance rate.
        if matched > 0 {
            self.metrics.speculation_accepted_tokens.add(
                &opentelemetry::Context::current(),
                matched as u64,
                &self.metrics.attributes,
            );
        }

        // Drop tentative draft cells beyond what was committed, so KV holds
        // exactly n_past cells whether the run was accepted, rejected, or ended.
        let n_past = slots[slot_idx]
            .sequence
            .as_ref()
            .map(|seq| seq.n_past as i32)
            .unwrap_or(0);
        context.remove_sequence_from(slot_idx as i32, n_past);
        if let Some(seq) = slots[slot_idx].sequence.as_mut() {
            seq.pending_drafts.clear();
        }

        if let Some((reason, token)) = finished {
            results.push((slot_idx, token, Some(reason), n_past));
            return;
        }

        // Not completed: record the continuing token and propose the next drafts
        // (now including that token) for the combined decode to verify next
        // iteration -- from the draft model when configured, else prompt-lookup.
        if let Some(continuing) = continuing_token {
            if let Some(draft) = draft.as_mut() {
                let proposed = {
                    let seq = slots[slot_idx].sequence.as_mut().unwrap();
                    seq.spec_tokens.push(continuing);
                    draft.propose(slot_idx, &seq.spec_tokens)
                };
                if let Some(seq) = slots[slot_idx].sequence.as_mut() {
                    seq.pending_drafts = proposed;
                }
            } else if let Some(seq) = slots[slot_idx].sequence.as_mut() {
                if let Some(lookup) = seq.ngram.as_mut() {
                    lookup.push(continuing);
                    seq.pending_drafts = lookup.propose();
                }
            }
            results.push((slot_idx, continuing, None, n_past));
        }
    }

    async fn check_stop_sequences(&self, slot: &mut Slot) -> StopMatch {
        let Some(seq) = &slot.sequence else {
            return StopMatch::None;
        };

        if seq.generated_tokens.is_empty() {
            return StopMatch::None;
        }

        // Token-level stop sequences are always complete (full) matches.
        if let Some(len) = seq.stop_matcher.check_token_stop(&seq.generated_tokens) {
            let suffix_bytes: usize = seq.piece_byte_lengths.iter().rev().take(len).sum();
            let byte_pos = seq.generated_text.len().saturating_sub(suffix_bytes);
            return StopMatch::Full {
                byte_pos,
                tokens_to_remove: len,
            };
        }

        // String-level stop sequences against the incremental text buffer. A full
        // match farther back would already have been caught on an earlier token,
        // so scanning only the tail is sufficient (llama.cpp algorithm).
        Self::find_tail_stop(seq)
    }

    /// Scan the tail of the incremental text buffer for a string stop match. The
    /// window spans the longest configured stop string plus the last piece plus a
    /// margin, which always contains any possible match because every token is
    /// checked as it lands.
    ///
    /// Two passes, mirroring llama.cpp's server: first a full-substring search so
    /// a complete stop embedded inside the last piece with trailing bytes
    /// (e.g. a piece `"</s>\n"`) is caught, taking the earliest such match and
    /// classifying it as [`StopMatch::Full`]; then the suffix-anchored partial
    /// search, whose hit is [`StopMatch::Partial`] (held back, not terminated).
    fn find_tail_stop(seq: &ActiveSequence) -> StopMatch {
        let max_stop_bytes = seq.stop_matcher.max_string_pattern_bytes();
        if max_stop_bytes == 0 {
            return StopMatch::None;
        }

        let generated_text = &seq.generated_text;
        let last_piece_len = seq.piece_byte_lengths.last().copied().unwrap_or(0);
        let raw_start = generated_text
            .len()
            .saturating_sub(max_stop_bytes + last_piece_len + STOP_SCAN_MARGIN);
        let window_start = floor_char_boundary(generated_text, raw_start);
        let tail = &generated_text[window_start..];

        let mut earliest_full: Option<usize> = None;
        for stop_string in seq.stop_matcher.string_patterns() {
            if let Some(full_pos) = stop_sequence::find_full_stop(tail, stop_string) {
                let absolute = window_start + full_pos;
                earliest_full =
                    Some(earliest_full.map_or(absolute, |existing| existing.min(absolute)));
            }
        }
        if let Some(byte_pos) = earliest_full {
            let suffix_len = generated_text.len() - byte_pos;
            let tokens_to_remove = count_tokens_for_suffix(&seq.piece_byte_lengths, suffix_len);
            return StopMatch::Full {
                byte_pos,
                tokens_to_remove,
            };
        }

        // A complete stop anchored at the text end was already caught by the
        // full-substring pass above, so any partial hit here is a strict prefix.
        for stop_string in seq.stop_matcher.string_patterns() {
            if let Some(partial_pos) = stop_sequence::find_partial_stop(tail, stop_string) {
                return StopMatch::Partial {
                    byte_pos: window_start + partial_pos,
                };
            }
        }

        StopMatch::None
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
        stop_match: &StopMatch,
        max_tokens: usize,
    ) -> Option<CompletionReason> {
        if seq.n_past as i32 >= self.n_ctx_seq - 1 {
            return Some(CompletionReason::MaxTokens {
                partial_stop: false,
            });
        }

        if is_eog {
            return Some(CompletionReason::EndOfGeneration);
        }

        // Only a full match terminates generation; a partial match is held back
        // and generation continues.
        if let StopMatch::Full {
            tokens_to_remove, ..
        } = stop_match
        {
            return Some(CompletionReason::StopSequence(*tokens_to_remove));
        }

        if seq.generated_tokens.len() >= max_tokens {
            // At the token budget, a partial stop at the end is trimmed (e.g. "<"
            // when stop is "</s>"), keeping partial matching's current use here.
            let partial_stop = matches!(stop_match, StopMatch::Partial { .. });
            return Some(CompletionReason::MaxTokens { partial_stop });
        }

        None
    }
}

/// Count the minimal number of trailing tokens whose combined byte length
/// covers the last `suffix_len` bytes of the generated text. Replaces the
/// binary-search re-detokenization by walking the recorded piece lengths.
fn count_tokens_for_suffix(piece_byte_lengths: &[usize], suffix_len: usize) -> usize {
    let mut bytes = 0;
    let mut count = 0;
    for &piece_len in piece_byte_lengths.iter().rev() {
        if bytes >= suffix_len {
            break;
        }
        bytes += piece_len;
        count += 1;
    }
    count
}

/// Length of the longest shared leading run of two token slices, used to score
/// prompt-cache reuse against an idle slot's retained tokens.
fn common_prefix_len(a: &[i32], b: &[i32]) -> usize {
    a.iter().zip(b.iter()).take_while(|(x, y)| x == y).count()
}

/// Round `index` down to the nearest UTF-8 char boundary of `text`, mirroring
/// the unstable `str::floor_char_boundary`.
fn floor_char_boundary(text: &str, index: usize) -> usize {
    if index >= text.len() {
        return text.len();
    }
    let mut boundary = index;
    while boundary > 0 && !text.is_char_boundary(boundary) {
        boundary -= 1;
    }
    boundary
}
