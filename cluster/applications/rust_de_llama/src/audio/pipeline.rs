//! Audio generation pipeline of the Thinker-Talker architecture: input text
//! -> thinker hidden states -> trained projection -> talker embedding space ->
//! autoregressive audio codes -> WavTokenizer decoder -> waveform. All stages
//! run synchronously on the CPU thread that calls `generate` (the handler wraps
//! it in `spawn_blocking`).

/// The thinker half of the architecture, with the projection that carries its
/// hidden states into the talker. The two are meaningless apart, and a pipeline
/// without them lets the talker read the text itself.
struct ProjectedThinker {
    /// Shared with the chat path when that model is already resident, so the
    /// same GGUF is not loaded twice.
    model: std::sync::Arc<rust_de_llama::LlamaModel>,
    context: rust_de_llama::LlamaContext,
    tokenizer: crate::parallel::tokenizer::Tokenizer,
    batch_buffer: crate::parallel::batch_buffer::BatchBuffer,
    projection: crate::audio::projection::ProjectionModel,
    n_ctx: usize,
    /// Tokens per decode call. llama.cpp reserves an output buffer sized
    /// `n_outputs * n_vocab` logits even when only embeddings are read, so the
    /// prompt is decoded in micro-batch-sized chunks to bound that transient.
    decode_chunk: usize,
}

/// Words per synthesis chunk. `MAX_CODES` bounds one run at ~27 s, and speech
/// runs about 2.5 words/second, so ~68 words is where a run is cut off; this
/// leaves room for words that take longer than average rather than truncating
/// them, at the cost of more runs.
const WORDS_PER_CHUNK: usize = 40;

/// Characters one request may ask for. The per-chunk length check bounds a
/// single run, but nothing bounded how many runs a request could ask for:
/// chunk count followed from the input, so axum's 2 MB body limit was the only
/// stop, and it admits thousands of chunks -- hours of audio, gigabytes of
/// samples, and the pipeline's mutex held for all of it while every other
/// request for that voice waits. This is OpenAI's own limit for
/// `/v1/audio/speech`, which this endpoint mirrors.
///
/// Enforced here because both endpoints reach the talker only through
/// `generate`, so this is the one place neither can get past. `audio_speech`
/// checks it again up front, where it can refuse before a pipeline is loaded.
pub const MAX_INPUT_CHARACTERS: usize = 4096;

/// Samples of overlap between consecutive runs. `embd_to_audio` trims its
/// padding, so a run begins and ends on a non-zero sample and the envelope
/// normalization is weakest exactly there; butting two together steps the
/// waveform audibly. 5 ms is long enough to ramp between them and far shorter
/// than the pause a sentence break already carries, so it removes the step
/// without smearing the break.
const CROSSFADE_SAMPLES: usize = crate::audio::wavtokenizer::SAMPLE_RATE as usize / 200;

/// Split text into runs of at most `WORDS_PER_CHUNK` words, preferring to break
/// where a sentence ends. Splitting happens before normalization because that
/// is what still has the punctuation to break on, and a sentence end is where a
/// pause is natural -- so the runs can be rendered independently and
/// concatenated without sounding cut.
fn split_for_synthesis(text: &str) -> Vec<String> {
    let mut chunks = Vec::new();
    let mut chunk = String::new();
    let mut words = 0usize;

    for sentence in split_sentences(text) {
        for run in split_words(sentence.trim(), WORDS_PER_CHUNK) {
            let run_words = run.split_whitespace().count();
            if run_words == 0 {
                continue;
            }
            if words > 0 && words + run_words > WORDS_PER_CHUNK {
                chunks.push(std::mem::take(&mut chunk));
                words = 0;
            }
            if !chunk.is_empty() {
                chunk.push(' ');
            }
            chunk.push_str(&run);
            words += run_words;
        }
    }
    if !chunk.is_empty() {
        chunks.push(chunk);
    }
    chunks
}

/// Break a sentence with no pause to break at into runs of at most `limit`
/// words. Breaking mid-clause is audible, but it is the lesser fault: a run the
/// talker cannot finish is cut off mid-word with nothing in the response or the
/// samples to say so.
fn split_words(sentence: &str, limit: usize) -> Vec<String> {
    let words: Vec<&str> = sentence.split_whitespace().collect();
    if words.len() <= limit {
        return Vec::from([sentence.to_string()]);
    }
    // Rejoining on single spaces loses the original spacing, which nothing
    // downstream reads: the talker's normalization collapses it anyway.
    words.chunks(limit).map(|chunk| chunk.join(" ")).collect()
}

/// Split at sentence-ending punctuation, keeping it with the sentence it ends.
fn split_sentences(text: &str) -> Vec<&str> {
    let mut sentences = Vec::new();
    let mut start = 0;
    for (index, character) in text.char_indices() {
        if matches!(character, '.' | '!' | '?' | '\n') {
            let end = index + character.len_utf8();
            if !text[start..end].trim().is_empty() {
                sentences.push(&text[start..end]);
            }
            start = end;
        }
    }
    if !text[start..].trim().is_empty() {
        sentences.push(&text[start..]);
    }
    sentences
}

/// Per-pipeline instruments, mirroring `crate::parallel::Metrics`.
struct Metrics {
    /// Text positions fed to the talker, the audio path's equivalent of the
    /// chat path's `processed_tokens_total`.
    text_positions: opentelemetry::metrics::Counter<u64>,
    /// Codes generated.
    codes: opentelemetry::metrics::Counter<u64>,
    /// Runs the talker did not finish on its own. `codes` against `MAX_CODES`
    /// cannot separate "hit the cap" from "finished", and the samples of a
    /// truncated run look exactly like a short one, so the count has to be
    /// carried rather than inferred.
    truncations: opentelemetry::metrics::Counter<u64>,
    /// Wall time of a whole `generate`, which is seconds of CPU-bound work.
    duration: opentelemetry::metrics::Histogram<f64>,
    attributes: [opentelemetry::KeyValue; 1],
}

impl Metrics {
    fn new(meter: &opentelemetry::metrics::Meter, pipeline_name: &str) -> Self {
        Self {
            text_positions: meter
                .u64_counter("audio_text_positions_total")
                .with_description("Total text positions fed to the talker")
                .init(),
            codes: meter
                .u64_counter("audio_codes_total")
                .with_description("Total audio codes generated")
                .init(),
            truncations: meter
                .u64_counter("audio_truncated_runs_total")
                .with_description("Runs cut off by the code limit before the talker stopped")
                .init(),
            duration: meter
                .f64_histogram("audio_generation_duration_seconds")
                .with_description("Wall time of one audio generation")
                .init(),
            attributes: [opentelemetry::KeyValue::new(
                "pipeline",
                pipeline_name.to_string(),
            )],
        }
    }
}

pub struct AudioPipeline {
    /// Absent in talker-native synthesis, where the text reaches the talker as
    /// its own tokens instead of as projected hidden states.
    thinker: Option<ProjectedThinker>,
    talker: crate::audio::talker::Talker,
    decoder: crate::audio::wavtokenizer::WavTokenizer,
    metrics: Metrics,
    temperature: f32,
}

pub struct AudioOutput {
    /// Mono PCM samples in [-1, 1].
    pub samples: Vec<f32>,
    pub sample_rate: u32,
    /// False when some run hit the code limit before the talker stopped, so
    /// the speech is cut mid-utterance. Chunking is sized to avoid this, but a
    /// single unbroken run can still outrun `MAX_CODES`, and nothing in
    /// `samples` distinguishes that from an utterance that simply ended.
    pub complete: bool,
}

/// What one `generate` spent, kept apart from its result so the metrics are
/// recorded whether or not a chunk failed. The first error used to propagate
/// straight out with `?`, which threw away the finished chunks' work and
/// skipped the metrics entirely -- so the most expensive runs were the least
/// observable ones.
#[derive(Default)]
struct Accounting {
    text_positions: usize,
    codes: usize,
    truncations: usize,
}

/// Distinguishes client-correctable input problems (mapped to 400) from
/// internal pipeline failures (mapped to 500).
pub enum GenerateError {
    InvalidInput(String),
    Internal(error::Error),
}

impl std::fmt::Display for GenerateError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            GenerateError::InvalidInput(message) => formatter.write_str(message),
            GenerateError::Internal(e) => write!(formatter, "{e}"),
        }
    }
}

impl ProjectedThinker {
    #[tracing::instrument(skip(talker, loaded_thinker))]
    fn load(
        thinker_path: &std::path::Path,
        projection_path: &std::path::Path,
        talker: &crate::audio::talker::Talker,
        n_ctx: i32,
        n_threads: i32,
        loaded_thinker: Option<std::sync::Arc<rust_de_llama::LlamaModel>>,
    ) -> Result<Self, error::Error> {
        let model = match loaded_thinker {
            Some(model) => {
                tracing::info!(
                    "Sharing already-loaded weights for thinker '{}'",
                    thinker_path.display()
                );
                model
            }
            None => std::sync::Arc::new(
                rust_de_llama::LlamaModel::load_from_file(thinker_path, None, None, false)
                    .map_err(|e| {
                        error::error!(
                            "Failed to load thinker model '{}': {}",
                            thinker_path.display(),
                            e
                        )
                    })?,
            ),
        };
        // Hidden states are collected per micro-batch-sized decode call, so
        // the batch only needs to span one chunk.
        let decode_chunk = 512.min(n_ctx);
        let context = rust_de_llama::LlamaContext::new_with_embeddings(
            &model,
            n_ctx,
            decode_chunk,
            decode_chunk,
            n_threads,
            n_threads,
        )
        .map_err(|e| error::error!("Failed to create thinker embeddings context: {}", e))?;
        // An embeddings context is its own context over the same weights, so it
        // can coexist with the chat path's generation contexts.

        let projection =
            crate::audio::projection::ProjectionModel::load_from_gguf(projection_path)?;
        let n_embd = model.n_embd();
        // llama.cpp strides its embeddings buffer by n_embd_out; hidden-state
        // reads assume it equals n_embd, so reject models where it differs.
        if model.n_embd_out() != n_embd {
            return Err(error::error!(
                "Thinker '{}' has n_embd_out {} != n_embd {}, which the hidden-state reader does not support",
                thinker_path.display(),
                model.n_embd_out(),
                n_embd
            ));
        }
        // The projection bridges two fixed endpoints: it reads the thinker's
        // hidden states and stands in for the talker's own token embeddings.
        if projection.input_dim != n_embd as usize {
            return Err(error::error!(
                "Projection input_dim {} does not match thinker '{}' n_embd {}",
                projection.input_dim,
                thinker_path.display(),
                n_embd
            ));
        }
        if projection.output_dim != talker.n_embd() as usize {
            return Err(error::error!(
                "Projection output_dim {} does not match the talker's n_embd {}",
                projection.output_dim,
                talker.n_embd()
            ));
        }

        Ok(Self {
            tokenizer: crate::parallel::tokenizer::Tokenizer::new(n_ctx as usize),
            batch_buffer: crate::parallel::batch_buffer::BatchBuffer::new(decode_chunk as usize),
            model,
            context,
            projection,
            n_ctx: n_ctx as usize,
            decode_chunk: decode_chunk as usize,
        })
    }

    /// Run the text through the thinker and project every hidden state into the
    /// talker's embedding space.
    fn project(&mut self, input: &str, limit: usize) -> Result<Vec<f32>, GenerateError> {
        let tokens = self
            .tokenizer
            .tokenize(self.model.get_vocab(), input)
            .map_err(|e| GenerateError::InvalidInput(format!("Failed to tokenize input: {e}")))?;
        if tokens.is_empty() {
            return Err(GenerateError::InvalidInput(
                "Input produced no tokens".to_string(),
            ));
        }
        let limit = limit.min(self.n_ctx);
        if tokens.len() > limit {
            return Err(GenerateError::InvalidInput(format!(
                "Input is too long: {} tokens exceeds the limit of {}",
                tokens.len(),
                limit
            )));
        }

        let hidden_states = read_hidden_states(
            &mut self.context,
            &mut self.batch_buffer,
            &tokens,
            self.projection.input_dim,
            self.decode_chunk,
        )
        .map_err(GenerateError::Internal)?;

        self.projection
            .forward(&hidden_states)
            .map_err(GenerateError::Internal)
    }
}

/// Run `tokens` through a thinker's embeddings context and collect every
/// position's hidden state, flattened `[n_tokens, n_embd]`.
///
/// A fresh single-sequence causal pass, decoded in micro-batch-sized chunks;
/// every token requests output, and each chunk's states are copied out before
/// the next decode call invalidates the context's output buffer. Free-standing
/// so the consistency test drives this rather than a copy of it: what it checks
/// is exactly whether these states match the full-precision ones the projection
/// was trained on.
fn read_hidden_states(
    context: &mut rust_de_llama::LlamaContext,
    batch_buffer: &mut crate::parallel::batch_buffer::BatchBuffer,
    tokens: &[i32],
    n_embd: usize,
    decode_chunk: usize,
) -> Result<Vec<f32>, error::Error> {
    context.clear_sequence(0);
    let mut hidden_states = Vec::with_capacity(tokens.len() * n_embd);
    for (chunk_index, chunk) in tokens.chunks(decode_chunk).enumerate() {
        batch_buffer.reset();
        for (offset, &token) in chunk.iter().enumerate() {
            let position = chunk_index * decode_chunk + offset;
            batch_buffer.add_token(token, position as i32, 0, 1);
        }
        let batch = batch_buffer.as_llama_batch();
        context
            .decode(batch)
            .map_err(|e| error::error!("Thinker decode failed: {}", e))?;
        context.synchronize();

        let chunk_states = context
            .embeddings(chunk.len(), n_embd)
            .map_err(|e| error::error!("Failed to read thinker hidden states: {}", e))?;
        hidden_states.extend_from_slice(&chunk_states);
    }
    Ok(hidden_states)
}

impl AudioPipeline {
    #[tracing::instrument(skip(config, meter, loaded_thinker))]
    pub fn load(
        model_directory: &str,
        pipeline_name: &str,
        config: &crate::config::AudioPipelineConfig,
        n_threads: i32,
        meter: &opentelemetry::metrics::Meter,
        loaded_thinker: Option<std::sync::Arc<rust_de_llama::LlamaModel>>,
    ) -> Result<Self, error::Error> {
        let resolve = |file_name: &str| -> Result<std::path::PathBuf, error::Error> {
            let mut components = std::path::Path::new(file_name).components();
            let is_single_normal =
                matches!(components.next(), Some(std::path::Component::Normal(_)))
                    && components.next().is_none();
            let path = std::path::Path::new(model_directory).join(file_name);
            if !is_single_normal || !path.exists() {
                return Err(error::error!(
                    "Model file '{}' not found in {}",
                    file_name,
                    model_directory
                ));
            }
            Ok(path)
        };

        // The talker's context holds the speaker scaffolding, the text and every
        // generated code, so it is sized independently of the thinker's n_ctx.
        let talker = crate::audio::talker::Talker::load(
            &resolve(&config.talker_model)?,
            config.talker_n_ctx.max(1),
            n_threads,
        )?;
        let decoder = crate::audio::wavtokenizer::WavTokenizer::load(
            &resolve(&config.audio_decoder)?,
            n_threads,
        )?;

        let thinker = match (&config.thinker_model, &config.projection_model) {
            (Some(thinker_model), Some(projection_model)) => Some(ProjectedThinker::load(
                &resolve(thinker_model)?,
                &resolve(projection_model)?,
                &talker,
                config.n_ctx.max(1),
                n_threads,
                loaded_thinker,
            )?),
            _ => None,
        };

        tracing::info!(
            "Loaded audio pipeline '{}' ({})",
            pipeline_name,
            if thinker.is_some() {
                "projected thinker"
            } else {
                "talker-native"
            }
        );

        Ok(Self {
            thinker,
            talker,
            decoder,
            metrics: Metrics::new(meter, pipeline_name),
            temperature: config.temperature,
        })
    }

    /// Synthesize `input`, splitting it into runs the talker can finish and
    /// concatenating what they render.
    #[tracing::instrument(skip(self, input))]
    pub fn generate(
        &mut self,
        input: &str,
        temperature: Option<f32>,
        seed: Option<u32>,
    ) -> Result<AudioOutput, GenerateError> {
        // Bounded before anything is split or loaded: this is what keeps one
        // request from occupying the pipeline for hours.
        if input.chars().count() > MAX_INPUT_CHARACTERS {
            return Err(GenerateError::InvalidInput(format!(
                "Input is too long: {} characters exceeds the limit of {}",
                input.chars().count(),
                MAX_INPUT_CHARACTERS
            )));
        }
        let chunks = split_for_synthesis(input);
        if chunks.is_empty() {
            return Err(GenerateError::InvalidInput(
                "Input has nothing to speak".to_string(),
            ));
        }

        let started = std::time::Instant::now();
        let mut accounting = Accounting::default();
        let result = self.synthesize_all(&chunks, temperature, seed, &mut accounting);

        let context = opentelemetry::Context::current();
        self.metrics.text_positions.add(
            &context,
            accounting.text_positions as u64,
            &self.metrics.attributes,
        );
        self.metrics
            .codes
            .add(&context, accounting.codes as u64, &self.metrics.attributes);
        self.metrics.truncations.add(
            &context,
            accounting.truncations as u64,
            &self.metrics.attributes,
        );
        self.metrics.duration.record(
            &context,
            started.elapsed().as_secs_f64(),
            &self.metrics.attributes,
        );

        Ok(AudioOutput {
            samples: result?,
            sample_rate: crate::audio::wavtokenizer::SAMPLE_RATE,
            complete: accounting.truncations == 0,
        })
    }

    /// Render every chunk and join them. Separate from `generate` so its `?`
    /// unwinds only to there, where the metrics have already been recorded.
    fn synthesize_all(
        &mut self,
        chunks: &[String],
        temperature: Option<f32>,
        seed: Option<u32>,
        accounting: &mut Accounting,
    ) -> Result<Vec<f32>, GenerateError> {
        let mut samples = Vec::new();
        for chunk in chunks {
            let rendered = self.synthesize_chunk(chunk, temperature, seed, accounting)?;
            append_with_crossfade(&mut samples, &rendered);
        }
        Ok(samples)
    }

    /// One run through the talker and the decoder, charging what it spent to
    /// `accounting` as it goes so a later failure does not lose the record.
    fn synthesize_chunk(
        &mut self,
        chunk: &str,
        temperature: Option<f32>,
        seed: Option<u32>,
        accounting: &mut Accounting,
    ) -> Result<Vec<f32>, GenerateError> {
        // Every text position, projected or tokenized, has to sit in the
        // talker's context alongside the speaker scaffolding and the codes.
        // Exceeding it is client-correctable, so it is caught before the
        // thinker pass and the projection's matmuls run.
        let limit = self
            .talker
            .max_text_positions(crate::audio::wavtokenizer::MAX_CODES);
        let prefix_tokens = self.talker.prompt_prefix_tokens().to_vec();
        let suffix_tokens = self.talker.prompt_suffix_tokens().to_vec();

        // The text reaches the talker either as projected thinker hidden states
        // or as its own tokens; both occupy the same slot between the speaker
        // scaffolding that conditions the voice.
        let projected = match self.thinker.as_mut() {
            Some(thinker) => Some(thinker.project(chunk, limit)?),
            None => None,
        };
        let text_tokens = match projected {
            Some(_) => Vec::new(),
            None => {
                let tokens = self
                    .talker
                    .prompt_text_tokens(chunk)
                    .map_err(|e| GenerateError::InvalidInput(format!("{e}")))?;
                if tokens.len() > limit {
                    return Err(GenerateError::InvalidInput(format!(
                        "Input is too long: {} tokens exceeds the limit of {}",
                        tokens.len(),
                        limit
                    )));
                }
                tokens
            }
        };
        let text_segment = match &projected {
            Some(projected) => crate::audio::talker::PrefixSegment::Embeddings(projected),
            None => crate::audio::talker::PrefixSegment::Tokens(&text_tokens),
        };
        let text_positions = match &projected {
            Some(projected) => projected.len() / self.talker.n_embd() as usize,
            None => text_tokens.len(),
        };

        let output = self
            .talker
            .generate(
                &[
                    crate::audio::talker::PrefixSegment::Tokens(&prefix_tokens),
                    text_segment,
                    crate::audio::talker::PrefixSegment::Tokens(&suffix_tokens),
                ],
                crate::audio::wavtokenizer::MAX_CODES,
                temperature.unwrap_or(self.temperature),
                seed,
            )
            .map_err(GenerateError::Internal)?;
        accounting.text_positions += text_positions;
        accounting.codes += output.codes.len();
        // Chunking is sized to avoid this, so reaching the cap means one run
        // outran it -- the audio stops mid-utterance and nothing in the samples
        // shows it. It reaches the caller as `AudioOutput::complete` and the
        // truncation counter; the log line names the run those cannot.
        if !output.complete {
            accounting.truncations += 1;
            tracing::warn!(
                "Talker hit its code limit before finishing; audio for this run is cut short: {:?}",
                chunk
            );
        }
        self.decoder
            .decode(&output.codes)
            .map_err(GenerateError::Internal)
    }
}

/// Append `chunk`, ramping across the seam rather than stepping at it.
fn append_with_crossfade(samples: &mut Vec<f32>, chunk: &[f32]) {
    let overlap = CROSSFADE_SAMPLES.min(samples.len()).min(chunk.len());
    if overlap == 0 {
        samples.extend_from_slice(chunk);
        return;
    }

    // Linear ramp: the two runs are independent utterances rather than two
    // takes of one sound, so there is no phase to preserve and an equal-power
    // curve would only raise the seam's loudness.
    let seam = samples.len() - overlap;
    for offset in 0..overlap {
        let weight = (offset + 1) as f32 / (overlap + 1) as f32;
        samples[seam + offset] = samples[seam + offset] * (1.0 - weight) + chunk[offset] * weight;
    }
    samples.extend_from_slice(&chunk[overlap..]);
}

/// Clamp to [-1, 1] and quantize to interleaved little-endian signed 16-bit.
pub fn encode_pcm16(samples: &[f32]) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(samples.len() * 2);
    for &sample in samples {
        let value = (sample.clamp(-1.0, 1.0) * 32767.0) as i16;
        bytes.extend_from_slice(&value.to_le_bytes());
    }
    bytes
}

/// Minimal RIFF/WAVE container: mono 16-bit PCM.
pub fn encode_wav(samples: &[f32], sample_rate: u32) -> Vec<u8> {
    let data = encode_pcm16(samples);
    let byte_rate = sample_rate * 2;
    let mut bytes = Vec::with_capacity(44 + data.len());
    bytes.extend_from_slice(b"RIFF");
    bytes.extend_from_slice(&(36 + data.len() as u32).to_le_bytes());
    bytes.extend_from_slice(b"WAVE");
    bytes.extend_from_slice(b"fmt ");
    bytes.extend_from_slice(&16u32.to_le_bytes());
    bytes.extend_from_slice(&1u16.to_le_bytes()); // PCM
    bytes.extend_from_slice(&1u16.to_le_bytes()); // mono
    bytes.extend_from_slice(&sample_rate.to_le_bytes());
    bytes.extend_from_slice(&byte_rate.to_le_bytes());
    bytes.extend_from_slice(&2u16.to_le_bytes()); // block align
    bytes.extend_from_slice(&16u16.to_le_bytes()); // bits per sample
    bytes.extend_from_slice(b"data");
    bytes.extend_from_slice(&(data.len() as u32).to_le_bytes());
    bytes.extend_from_slice(&data);
    bytes
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_split_for_synthesis_groups_sentences_under_the_budget() {
        let text = "One two three. Four five six! Seven eight?";
        assert_eq!(
            split_for_synthesis(text),
            Vec::from(["One two three. Four five six! Seven eight?".to_string()])
        );
    }

    #[test]
    fn test_split_for_synthesis_breaks_at_sentence_ends() {
        // Two sentences of 30 words each cannot share a 40-word chunk.
        let sentence = format!("{}.", vec!["word"; 30].join(" "));
        let chunks = split_for_synthesis(&format!("{sentence} {sentence}"));
        assert_eq!(chunks.len(), 2);
        assert!(chunks
            .iter()
            .all(|chunk| chunk.split_whitespace().count() == 30));
    }

    /// A single sentence with no pause to break at must still be broken: left
    /// whole it outruns a run and is cut off mid-word, which nothing in the
    /// response reports.
    #[test]
    fn test_split_for_synthesis_breaks_an_over_long_sentence() {
        let sentence = format!("{}.", vec!["word"; WORDS_PER_CHUNK * 3].join(" "));
        let chunks = split_for_synthesis(&sentence);
        assert_eq!(chunks.len(), 3);
        assert!(chunks
            .iter()
            .all(|chunk| chunk.split_whitespace().count() <= WORDS_PER_CHUNK));
        // Every word survives the break, in order.
        assert_eq!(
            chunks.join(" ").split_whitespace().count(),
            WORDS_PER_CHUNK * 3
        );
    }

    #[test]
    fn test_split_for_synthesis_ignores_empty_input() {
        assert!(split_for_synthesis("").is_empty());
        assert!(split_for_synthesis("   \n  ").is_empty());
        // Punctuation alone still yields a run: whether it is speakable is the
        // talker's call, not the splitter's, since the projected path never
        // normalizes the text at all.
        assert_eq!(split_for_synthesis("...").len(), 1);
    }

    /// Splitting walks byte indices; a multi-byte character must not panic or
    /// be cut in half.
    #[test]
    fn test_split_for_synthesis_handles_multibyte_text() {
        assert_eq!(
            split_for_synthesis("こんにちは。ありがとう。"),
            Vec::from(["こんにちは。ありがとう。".to_string()])
        );
        assert_eq!(
            split_for_synthesis("Hello 🎉. World 🎉!"),
            Vec::from(["Hello 🎉. World 🎉!".to_string()])
        );
    }

    /// The first run has nothing to ramp against, so it is copied whole.
    #[test]
    fn test_append_with_crossfade_copies_the_first_chunk() {
        let mut samples = Vec::new();
        append_with_crossfade(&mut samples, &[0.5; 100]);
        assert_eq!(samples, vec![0.5; 100]);
    }

    /// Two runs that each sit at a constant level must join at that level
    /// rather than stepping: the ramp is what the seam needs to be inaudible.
    #[test]
    fn test_append_with_crossfade_ramps_between_runs() {
        let mut samples = vec![1.0f32; CROSSFADE_SAMPLES * 2];
        append_with_crossfade(&mut samples, &[-1.0f32; CROSSFADE_SAMPLES * 2]);

        // No samples are lost: the overlap is faded in place, not dropped.
        assert_eq!(samples.len(), CROSSFADE_SAMPLES * 4 - CROSSFADE_SAMPLES);
        let seam = CROSSFADE_SAMPLES;
        assert_eq!(samples[seam - 1], 1.0);
        assert_eq!(samples[seam + CROSSFADE_SAMPLES], -1.0);
        // Monotone across the seam, so no step is left anywhere in it.
        for offset in 0..CROSSFADE_SAMPLES {
            assert!(
                samples[seam + offset] < samples[seam + offset - 1],
                "not monotone at {offset}"
            );
        }
    }

    /// A run shorter than the crossfade must not index past either buffer.
    #[test]
    fn test_append_with_crossfade_handles_runs_shorter_than_the_overlap() {
        let mut samples = vec![1.0f32; 3];
        append_with_crossfade(&mut samples, &[-1.0f32; 2]);
        assert_eq!(samples.len(), 3);
    }

    /// The runtime extracts the thinker's hidden states from a quantized GGUF;
    /// the projection was trained on the full-precision HF ones. Nothing made
    /// the two agree, and until this test neither half of that had been
    /// measured.
    ///
    /// The fixture is written by `notebooks/thinker-talker-projection.ipynb`
    /// (section 14) from the HF model in fp32: the token ids it saw, then one
    /// row of `hidden_states[-1]` per token.
    ///
    /// **Tokenization is the half that would be silent.** Different ids mean
    /// the states are not row-comparable at all, and every later number would
    /// be measuring the wrong pair. It is asserted exactly. Both ends land on
    /// the same ids because neither adds BOS -- the GGUF sets
    /// `tokenizer.ggml.add_bos_token = 0`, so llama.cpp adds none despite the
    /// runtime passing `add_special = true`, and SmolLM2's HF tokenizer adds
    /// none either.
    ///
    /// **Quantization is the half that is a matter of degree**, so it is
    /// reported and bounded rather than asserted equal.
    #[test]
    fn test_thinker_hidden_states_match_the_full_precision_reference() {
        let manifest = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
        let fixture_path = manifest.join("tests/fixtures/thinker_hidden_states.txt");
        if !fixture_path.exists() {
            eprintln!("skipped: {} is absent", fixture_path.display());
            return;
        }

        let fixture = std::fs::read_to_string(&fixture_path).unwrap();
        let mut lines = fixture.lines();
        let reference_tokens: Vec<i32> = lines
            .next()
            .unwrap()
            .split_whitespace()
            .map(|value| value.parse().unwrap())
            .collect();
        let reference: Vec<Vec<f32>> = lines
            .map(|line| {
                line.split_whitespace()
                    .map(|value| value.parse().unwrap())
                    .collect()
            })
            .collect();

        rust_de_llama::ensure_backend_init();
        // Both quantizations a thinker plausibly runs at. Q4_K_M is the one that
        // matters: it is what the chat models here use, so it is what a thinker
        // shared with the chat path is quantized to.
        let mut measured = 0;
        for (name, minimum_cosine) in [
            ("SmolLM2-135M-Instruct-Q8_0.gguf", 0.99f32),
            ("SmolLM2-135M-Instruct-Q4_K_M.gguf", 0.95f32),
        ] {
            let thinker_path = manifest.join("models").join(name);
            if !thinker_path.exists() {
                eprintln!("skipped: {} is absent", thinker_path.display());
                continue;
            }
            measured += 1;
            let model = rust_de_llama::LlamaModel::load_from_file(&thinker_path, None, None, false)
                .unwrap();
            let n_embd = model.n_embd() as usize;
            let decode_chunk = 512;
            let mut context = rust_de_llama::LlamaContext::new_with_embeddings(
                &model,
                2048,
                decode_chunk as i32,
                decode_chunk as i32,
                0,
                0,
            )
            .unwrap();
            let mut batch_buffer = crate::parallel::batch_buffer::BatchBuffer::new(decode_chunk);

            // The same call the runtime makes, so a drift in add_special would show.
            let tokens = crate::parallel::tokenizer::Tokenizer::new(2048)
                .tokenize(
                    model.get_vocab(),
                    "the quick brown fox jumps over the lazy dog",
                )
                .unwrap();
            assert_eq!(
            tokens, reference_tokens,
            "the GGUF vocab and the HF tokenizer disagree, so the projection would read positions it was never trained on"
        );

            let states = read_hidden_states(
                &mut context,
                &mut batch_buffer,
                &tokens,
                n_embd,
                decode_chunk,
            )
            .unwrap();
            assert_eq!(states.len(), reference.len() * n_embd);

            let mut worst_similarity = 1.0f32;
            let mut worst_relative = 0.0f32;
            for (index, expected) in reference.iter().enumerate() {
                let actual = &states[index * n_embd..(index + 1) * n_embd];
                let dot: f32 = actual.iter().zip(expected).map(|(a, b)| a * b).sum();
                let actual_norm = actual.iter().map(|v| v * v).sum::<f32>().sqrt();
                let expected_norm = expected.iter().map(|v| v * v).sum::<f32>().sqrt();
                let similarity = dot / (actual_norm * expected_norm);
                let distance = actual
                    .iter()
                    .zip(expected)
                    .map(|(a, b)| (a - b) * (a - b))
                    .sum::<f32>()
                    .sqrt();
                worst_similarity = worst_similarity.min(similarity);
                worst_relative = worst_relative.max(distance / expected_norm);
                eprintln!(
                    "  {name} position {index}: cosine {similarity:.6}, relative error {:.4}",
                    distance / expected_norm
                );
            }
            eprintln!(
            "{name} vs fp32: worst cosine {worst_similarity:.6}, worst relative error {worst_relative:.4}"
        );

            // Loose enough to be about quantization rather than the exact build,
            // tight enough that a wrong layer or a mis-strided read fails it: those
            // land near zero cosine, not near one.
            assert!(
            worst_similarity > minimum_cosine,
            "{name} hidden states diverge from the reference the projection trains on: cosine {worst_similarity}"
        );
        }
        assert!(measured > 0, "no thinker GGUF to measure against");
    }

    /// The projected path end to end: raw text -> thinker hidden states ->
    /// trained projection -> talker -> decoder. This is the half of the
    /// architecture `test_generate_native_speech` cannot reach, since that one
    /// hands the talker its own tokens and never involves the thinker.
    ///
    /// Skipped until the projection GGUF is there: it is the one stage this
    /// repository ships no weights for, and it is what
    /// `notebooks/thinker-talker-projection.ipynb` trains.
    ///
    /// What this asserts is only that the path runs and produces plausible
    /// audio. It cannot assert the words: a projection that carries no text
    /// still yields fluent speech, because the talker falls back on its own
    /// prior -- it just says something unrelated, at a perfectly ordinary
    /// length and RMS. Only transcription separates the two, which is
    /// `e2e.sh`'s job.
    #[test]
    fn test_generate_projected_speech() {
        let models = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("models");
        let config = crate::config::AudioPipelineConfig {
            thinker_model: Some("SmolLM2-135M-Instruct-Q8_0.gguf".to_string()),
            talker_model: "OuteTTS-0.2-500M-F16.gguf".to_string(),
            projection_model: Some("thinker-talker-projection.gguf".to_string()),
            audio_decoder: "WavTokenizer-Large-75-F16.gguf".to_string(),
            n_ctx: 2048,
            talker_n_ctx: 4096,
            temperature: 0.0,
        };
        for name in [
            config.thinker_model.as_deref().unwrap(),
            &config.talker_model,
            config.projection_model.as_deref().unwrap(),
            &config.audio_decoder,
        ] {
            if !models.join(name).exists() {
                eprintln!("skipped: {} is absent", models.join(name).display());
                return;
            }
        }

        let text = "the quick brown fox jumps over the lazy dog";
        rust_de_llama::ensure_backend_init();
        let mut pipeline = AudioPipeline::load(
            models.to_str().unwrap(),
            "test-projected",
            &config,
            0,
            &opentelemetry::global::meter("test"),
            None,
        )
        .unwrap();

        let output = pipeline
            .generate(text, Some(0.0), Some(42))
            .unwrap_or_else(|e| panic!("generate failed: {e}"));
        assert!(output.complete, "the run was cut off by the code limit");
        // Nine words cannot be spoken in under a second at 75 codes/second.
        assert!(
            output.samples.len() > crate::audio::wavtokenizer::SAMPLE_RATE as usize,
            "implausibly short: {} samples",
            output.samples.len()
        );
        let rms = (output.samples.iter().map(|s| s * s).sum::<f32>() / output.samples.len() as f32)
            .sqrt();
        assert!((0.005..=0.5).contains(&rms), "rms {rms}");

        let wav_path = std::env::temp_dir().join(format!(
            "rust_de_llama-projected-{}.wav",
            std::process::id()
        ));
        std::fs::write(&wav_path, encode_wav(&output.samples, output.sample_rate)).unwrap();
        eprintln!(
            "projected '{}' -> {} samples -> {}",
            text,
            output.samples.len(),
            wav_path.display()
        );
    }

    #[test]
    fn test_encode_pcm16_clamps_and_quantizes() {
        let bytes = encode_pcm16(&[0.0, 1.0, -1.0, 2.0]);
        assert_eq!(bytes.len(), 8);
        assert_eq!(i16::from_le_bytes([bytes[0], bytes[1]]), 0);
        assert_eq!(i16::from_le_bytes([bytes[2], bytes[3]]), 32767);
        assert_eq!(i16::from_le_bytes([bytes[4], bytes[5]]), -32767);
        assert_eq!(i16::from_le_bytes([bytes[6], bytes[7]]), 32767);
    }

    #[test]
    fn test_encode_wav_header() {
        let wav = encode_wav(&[0.0f32; 100], 24000);
        assert_eq!(&wav[0..4], b"RIFF");
        assert_eq!(&wav[8..12], b"WAVE");
        assert_eq!(&wav[12..16], b"fmt ");
        assert_eq!(u16::from_le_bytes([wav[22], wav[23]]), 1);
        assert_eq!(
            u32::from_le_bytes([wav[24], wav[25], wav[26], wav[27]]),
            24000
        );
        assert_eq!(&wav[36..40], b"data");
        assert_eq!(
            u32::from_le_bytes([wav[40], wav[41], wav[42], wav[43]]) as usize,
            200
        );
        assert_eq!(wav.len(), 44 + 200);
    }
}
