//! Talker: prefix -> audio-codec token sequence.
//!
//! The autoregressive half of the Thinker-Talker architecture. An OuteTTS-style
//! GGUF (a causal LM whose vocabulary embeds the decoder's codes as the token
//! run `<|0|>`..`<|4095|>`) is prefilled and then sampled until its
//! end-of-generation token; the audio-code tokens it emits map back to
//! WavTokenizer code ids for `crate::audio::wavtokenizer`.
//!
//! The prompt (ported from llama.cpp `tools/tts/tts.cpp`) wraps the text to
//! speak in an in-context voice example -- the speaker's words, the text to
//! speak, then the speaker's audio for its own words -- so the talker continues
//! with audio for the remaining text. The speaker example therefore defines the
//! voice, and is needed whether the text arrives as tokens or as a projected
//! prefix.

/// Reference speaker from llama.cpp `tools/tts/tts.cpp`, the default voice.
const SPEAKER_TEXT: &str = include_str!("talker/speaker_text.txt");
const SPEAKER_AUDIO: &str = include_str!("talker/speaker_audio.txt");

/// Codes the decoder's vocabulary defines; also the length of the talker's
/// `<|0|>`..`<|4095|>` token run.
const AUDIO_CODE_COUNT: i32 = 4096;

/// Text positions a context must leave once the speaker prompt and the codes
/// are seated, or it is not worth serving: `normalize_text` spends roughly two
/// tokens per word, so this is about a sentence.
const MINIMUM_TEXT_POSITIONS: usize = 32;

pub struct TalkerOutput {
    pub codes: Vec<i32>,
    /// Whether the talker stopped on its own end-of-generation token. False
    /// means a bound cut it off mid-utterance, which is not visible in `codes`:
    /// a truncated run looks exactly like a short one.
    pub complete: bool,
}

/// One run of prefill input. A `llama_batch` carries either tokens or
/// embeddings, so a prompt that mixes them (a projected prefix between the
/// speaker scaffolding) is decoded as consecutive segments into one sequence.
pub enum PrefixSegment<'a> {
    Tokens(&'a [i32]),
    /// Flattened `[n_positions, n_embd]` in the talker's embedding space.
    Embeddings(&'a [f32]),
}

impl PrefixSegment<'_> {
    fn positions(&self, n_embd: usize) -> Result<usize, error::Error> {
        match self {
            PrefixSegment::Tokens(tokens) => Ok(tokens.len()),
            PrefixSegment::Embeddings(embeddings) => {
                if !embeddings.len().is_multiple_of(n_embd) {
                    return Err(error::error!(
                        "Prefix embeddings length {} is not a multiple of the talker's n_embd {}",
                        embeddings.len(),
                        n_embd
                    ));
                }
                Ok(embeddings.len() / n_embd)
            }
        }
    }
}

pub struct Talker {
    model: rust_de_llama::LlamaModel,
    context: rust_de_llama::LlamaContext,
    batch_buffer: crate::parallel::batch_buffer::BatchBuffer,
    /// Token id of `<|0|>`; `<|n|>` is this plus `n`.
    audio_code_first_token: i32,
    /// Speaker scaffolding, tokenized once: it is constant per talker and
    /// ~900 tokens, so re-tokenizing it per request would cost more than the
    /// text it surrounds.
    prompt_prefix: Vec<i32>,
    prompt_suffix: Vec<i32>,
    n_ctx: usize,
    /// Positions per prefill decode call, bounding the micro-batch transient.
    prefill_chunk: usize,
}

impl Talker {
    #[tracing::instrument]
    pub fn load(path: &std::path::Path, n_ctx: i32, n_threads: i32) -> Result<Self, error::Error> {
        let model = rust_de_llama::LlamaModel::load_from_file(path, None, None, false)
            .map_err(|e| error::error!("Failed to load talker '{}': {}", path.display(), e))?;

        let n_ctx = n_ctx.max(1);
        let prefill_chunk = 512.min(n_ctx);
        let context = rust_de_llama::LlamaContext::new(
            &model,
            n_ctx,
            prefill_chunk,
            prefill_chunk,
            1,
            n_threads,
            n_threads,
            crate::config::KvCacheType::F16.to_ggml_type(),
            crate::config::KvCacheType::F16.to_ggml_type(),
            true,
        )
        .map_err(|e| error::error!("Failed to create talker context: {}", e))?;

        let mut tokenizer = crate::parallel::tokenizer::Tokenizer::new(n_ctx as usize);
        let audio_code_first_token = Self::resolve_audio_code_range(&model, &mut tokenizer, path)?;

        let vocab = model.get_vocab();
        // Only the leading segment may take a BOS; see `tokenize_special`.
        let prompt_prefix =
            tokenizer.tokenize_special(vocab, &format!("<|im_start|>\n{SPEAKER_TEXT}"), true)?;
        let prompt_suffix =
            tokenizer.tokenize_special(vocab, &format!("<|text_end|>\n{SPEAKER_AUDIO}"), false)?;
        let scaffolding = prompt_prefix.len() + prompt_suffix.len();
        // Reject a context that cannot seat the voice conditioning, the codes
        // and a useful amount of text, rather than accepting it and failing
        // every request once it is serving. The reserve mirrors what
        // `max_text_positions` subtracts, so the two cannot drift apart.
        let reserved = scaffolding + crate::audio::wavtokenizer::MAX_CODES;
        if reserved + MINIMUM_TEXT_POSITIONS > n_ctx as usize {
            return Err(error::error!(
                "Talker '{}' has n_ctx {} but its speaker prompt ({} positions) and {} codes leave under {} for text",
                path.display(),
                n_ctx,
                scaffolding,
                crate::audio::wavtokenizer::MAX_CODES,
                MINIMUM_TEXT_POSITIONS
            ));
        }

        tracing::info!(
            "Loaded talker '{}': n_embd={}, '<|0|>'={}, speaker prompt={} positions",
            path.display(),
            model.n_embd(),
            audio_code_first_token,
            scaffolding
        );

        Ok(Self {
            batch_buffer: crate::parallel::batch_buffer::BatchBuffer::new(prefill_chunk as usize),
            model,
            context,
            audio_code_first_token,
            prompt_prefix,
            prompt_suffix,
            n_ctx: n_ctx as usize,
            prefill_chunk: prefill_chunk as usize,
        })
    }

    /// Locate `<|0|>` and require `<|0|>`..`<|4095|>` to be one contiguous run,
    /// which avoids hard-coding vocabulary offsets and rejects a GGUF that is
    /// not a talker for this decoder.
    fn resolve_audio_code_range(
        model: &rust_de_llama::LlamaModel,
        tokenizer: &mut crate::parallel::tokenizer::Tokenizer,
        path: &std::path::Path,
    ) -> Result<i32, error::Error> {
        let vocab = model.get_vocab();
        let single_token = |tokenizer: &mut crate::parallel::tokenizer::Tokenizer,
                            marker: &str|
         -> Result<i32, error::Error> {
            let tokens = tokenizer.tokenize_special(vocab, marker, false)?;
            match tokens.as_slice() {
                [token] => Ok(*token),
                _ => Err(error::error!(
                    "Talker '{}' does not define '{}' as a single token, so it cannot drive this audio decoder",
                    path.display(),
                    marker
                )),
            }
        };

        let first = single_token(tokenizer, "<|0|>")?;
        // Endpoints alone would admit a permuted interior, and a code that maps
        // to the wrong token is silent corruption rather than an error.
        for code in 1..AUDIO_CODE_COUNT {
            if single_token(tokenizer, &format!("<|{code}|>"))? != first + code {
                return Err(error::error!(
                    "Talker '{}' does not encode audio codes contiguously: '<|{}|>' is not '<|0|>'+{}",
                    path.display(),
                    code,
                    code
                ));
            }
        }
        Ok(first)
    }

    pub fn n_embd(&self) -> i32 {
        self.model.n_embd()
    }

    /// Tokens preceding the text to speak: the turn opener and the speaker's
    /// words.
    pub fn prompt_prefix_tokens(&self) -> &[i32] {
        &self.prompt_prefix
    }

    /// Tokens following the text to speak: the text terminator and the
    /// speaker's audio, which the talker continues from.
    pub fn prompt_suffix_tokens(&self) -> &[i32] {
        &self.prompt_suffix
    }

    /// Positions left for the text to speak once the speaker scaffolding and
    /// `max_codes` are seated. Callers validate their input against this so an
    /// over-long request is refused up front rather than after the thinker and
    /// the projection have already run.
    pub fn max_text_positions(&self, max_codes: usize) -> usize {
        self.n_ctx
            .saturating_sub(self.prompt_prefix.len() + self.prompt_suffix.len() + max_codes)
    }

    /// Tokens for the text to speak itself, normalized into the talker's
    /// word-separated form. This is the talker-native path: it needs no trained
    /// projection, at the cost of confining the pipeline to what this
    /// normalization and the talker's tokenizer can say.
    pub fn prompt_text_tokens(&mut self, text: &str) -> Result<Vec<i32>, error::Error> {
        let normalized = normalize_text(text);
        if normalized.is_empty() {
            return Err(error::error!(
                "Input has no speakable characters after normalization"
            ));
        }
        crate::parallel::tokenizer::Tokenizer::new(self.n_ctx).tokenize_special(
            self.model.get_vocab(),
            &normalized,
            false,
        )
    }

    /// Prefill `prefix`, then sample audio-code tokens until the talker emits
    /// end-of-generation or `max_codes` codes have been collected. Returns
    /// WavTokenizer code ids.
    #[tracing::instrument(skip(self, prefix))]
    pub fn generate(
        &mut self,
        prefix: &[PrefixSegment<'_>],
        max_codes: usize,
        temperature: f32,
        seed: Option<u32>,
    ) -> Result<TalkerOutput, error::Error> {
        let n_embd = self.model.n_embd() as usize;
        let mut n_prefix = 0usize;
        for segment in prefix {
            n_prefix += segment.positions(n_embd)?;
        }
        if n_prefix == 0 {
            return Err(error::error!("Cannot generate from an empty prefix"));
        }
        if n_prefix >= self.n_ctx {
            return Err(error::error!(
                "Prefix of {} positions leaves no room to generate within the talker's n_ctx {}",
                n_prefix,
                self.n_ctx
            ));
        }

        self.context.clear_sequence(0);
        self.prefill(prefix, n_prefix, n_embd)?;

        let sampler = build_sampler(temperature, seed)?;
        let vocab = self.model.get_vocab();
        let mut codes = Vec::with_capacity(max_codes);
        let mut position = n_prefix;
        let mut complete = false;

        // Bounded by both: the talker spends positions on the scaffolding it
        // emits between words (`<|t_0.08|>`, `<|code_start|>`, `<|code_end|>`),
        // so codes are strictly fewer than the positions they occupy and only
        // the position bound keeps the sequence inside the context.
        while codes.len() < max_codes && position < self.n_ctx {
            // Prefill and each step below request logits only on their last
            // position, so the sampler always reads row -1.
            let token = sampler.sample(&self.context, -1);
            if unsafe { rust_de_llama::llama_vocab_is_eog(vocab, token) } {
                complete = true;
                break;
            }
            if let Some(code) = self.token_to_code(token) {
                codes.push(code);
            }

            self.batch_buffer.reset();
            self.batch_buffer.add_token(token, position as i32, 0, 1);
            let batch = self.batch_buffer.as_llama_batch();
            self.context
                .decode(batch)
                .map_err(|e| error::error!("Talker decode failed: {}", e))?;
            self.context.synchronize();
            position += 1;
        }

        if codes.is_empty() {
            return Err(error::error!("Talker produced no audio codes"));
        }
        Ok(TalkerOutput { codes, complete })
    }

    fn prefill(
        &mut self,
        prefix: &[PrefixSegment<'_>],
        n_prefix: usize,
        n_embd: usize,
    ) -> Result<(), error::Error> {
        let mut position = 0usize;
        for segment in prefix {
            let positions = segment.positions(n_embd)?;
            for chunk_start in (0..positions).step_by(self.prefill_chunk) {
                let chunk_end = (chunk_start + self.prefill_chunk).min(positions);
                self.batch_buffer.reset();
                for offset in chunk_start..chunk_end {
                    // Only the prompt's final position is sampled from.
                    let logit = i8::from(position + offset == n_prefix - 1);
                    match segment {
                        PrefixSegment::Tokens(tokens) => self.batch_buffer.add_token(
                            tokens[offset],
                            (position + offset) as i32,
                            0,
                            logit,
                        ),
                        PrefixSegment::Embeddings(embeddings) => self.batch_buffer.add_embedding(
                            &embeddings[offset * n_embd..(offset + 1) * n_embd],
                            (position + offset) as i32,
                            0,
                            logit,
                        ),
                    }
                }
                let batch = self.batch_buffer.as_llama_batch();
                self.context
                    .decode(batch)
                    .map_err(|e| error::error!("Talker prefill failed: {}", e))?;
            }
            position += positions;
        }
        self.context.synchronize();
        Ok(())
    }

    /// Audio-code tokens map to decoder code ids; every other token the talker
    /// emits between words (`<|code_start|>`, the `<|t_0.08|>` durations) is
    /// prompt scaffolding that carries no code.
    ///
    /// llama.cpp `tts.cpp` filters on `151672..=155772`, which also admits
    /// `<|4096|>`..`<|4099|>` and the duration token `<|t_0.00|>`; those exceed
    /// the decoder's vocabulary, so only the codes it defines are kept.
    fn token_to_code(&self, token: i32) -> Option<i32> {
        let code = token - self.audio_code_first_token;
        (0..AUDIO_CODE_COUNT).contains(&code).then_some(code)
    }
}

const ONES: [&str; 20] = [
    "zero",
    "one",
    "two",
    "three",
    "four",
    "five",
    "six",
    "seven",
    "eight",
    "nine",
    "ten",
    "eleven",
    "twelve",
    "thirteen",
    "fourteen",
    "fifteen",
    "sixteen",
    "seventeen",
    "eighteen",
    "nineteen",
];
const TENS: [&str; 10] = [
    "", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety",
];

/// Spell a number below 1000, as llama.cpp `tts.cpp` `convert_less_than_thousand`.
fn spell_below_thousand(number: u32, out: &mut String) {
    debug_assert!(number < 1000, "spell_below_thousand({number})");
    let number = number as usize;
    if number >= 100 {
        out.push_str(ONES[number / 100]);
        out.push_str(" hundred ");
    }
    let rest = number % 100;
    if rest >= 20 {
        out.push_str(TENS[rest / 10]);
        if !rest.is_multiple_of(10) {
            out.push('-');
            out.push_str(ONES[rest % 10]);
        }
    } else if rest > 0 {
        out.push_str(ONES[rest]);
    }
}

/// Spell a run of digits, optionally with a fractional part, as llama.cpp
/// `tts.cpp` `number_to_words`: the integer part in words, then "point" and the
/// fraction digit by digit.
fn spell_number(digits: &str) -> String {
    let (integer_part, fraction_part) = match digits.split_once('.') {
        Some((integer, fraction)) => (integer, Some(fraction)),
        None => (digits, None),
    };

    let mut spelled = String::new();
    // Parsed as u32, as upstream parses into a signed int and gives up when it
    // does not fit: a run of digits that large is dropped rather than mis-said.
    // The width is load-bearing, not incidental -- it caps the billions digit at
    // 4, which is what keeps `spell_below_thousand` inside its table. A wider
    // type would hand it five-digit billions and index past the end.
    match integer_part.parse::<u32>() {
        Ok(0) => spelled.push_str("zero"),
        Ok(mut number) => {
            for (unit, name) in [
                (1_000_000_000u32, "billion"),
                (1_000_000, "million"),
                (1_000, "thousand"),
            ] {
                if number >= unit {
                    spell_below_thousand(number / unit, &mut spelled);
                    spelled.push(' ');
                    spelled.push_str(name);
                    spelled.push(' ');
                    number %= unit;
                }
            }
            if number > 0 {
                spell_below_thousand(number, &mut spelled);
            }
        }
        Err(_) => return " ".to_string(),
    }

    if let Some(fraction) = fraction_part {
        spelled.push_str(" point");
        for digit in fraction.chars() {
            match digit.to_digit(10) {
                Some(digit) => {
                    spelled.push(' ');
                    spelled.push_str(ONES[digit as usize]);
                }
                None => return " ".to_string(),
            }
        }
    }
    spelled
}

/// Replace every digit run with its spelling, as llama.cpp `tts.cpp`
/// `replace_numbers_with_words`. Without this the digits are dropped by the
/// non-letter filter and the number is silently not said at all.
fn spell_numbers(text: &str) -> String {
    let mut spelled = String::with_capacity(text.len());
    let mut rest = text;
    while let Some(start) = rest.find(|character: char| character.is_ascii_digit()) {
        spelled.push_str(&rest[..start]);
        let digits = &rest[start..];
        // A '.' continues the run only when a digit follows, so "3.5" is one
        // number while "42. Next" is not.
        let mut end = 0;
        let mut seen_point = false;
        for (index, character) in digits.char_indices() {
            if character.is_ascii_digit() {
                end = index + character.len_utf8();
            } else if character == '.'
                && !seen_point
                && digits[index + 1..].starts_with(|next: char| next.is_ascii_digit())
            {
                seen_point = true;
            } else {
                break;
            }
        }
        spelled.push_str(&spell_number(&digits[..end]));
        rest = &digits[end..];
    }
    spelled.push_str(rest);
    spelled
}

/// Normalize text into the talker's prompt form, as llama.cpp `tts.cpp`
/// `process_text` does: spell numbers out, lower-case, drop everything but
/// letters, and separate words with `<|text_sep|>`. Only English is supported,
/// matching upstream.
///
/// Dropping everything but letters is also what keeps `tokenize_special` from
/// resolving user text into control tokens; relaxing this filter means
/// stripping `<`, `|` and `>` on purpose instead of by side effect.
fn normalize_text(text: &str) -> String {
    let lowered: String = spell_numbers(text)
        .to_lowercase()
        .chars()
        .map(|character| match character {
            '-' | '_' | '/' | ',' | '.' | '\\' => ' ',
            character if character.is_ascii_alphabetic() || character.is_whitespace() => character,
            _ => '\0',
        })
        .filter(|&character| character != '\0')
        .collect();

    lowered
        .split_whitespace()
        .map(|word| format!("{word}<|text_sep|>"))
        .collect()
}

/// Sampler chain in the order `crate::parallel::slot` establishes: greedy when
/// temperature is disabled, otherwise temperature then a seeded distribution.
fn build_sampler(
    temperature: f32,
    seed: Option<u32>,
) -> Result<rust_de_llama::LlamaSampler, error::Error> {
    if temperature <= 0.0 {
        return rust_de_llama::LlamaSampler::new_greedy()
            .map_err(|e| error::error!("Failed to create talker sampler: {}", e));
    }

    let chain = rust_de_llama::LlamaSampler::new()
        .map_err(|e| error::error!("Failed to create talker sampler: {}", e))?;
    let temperature_sampler = rust_de_llama::llama_sampler_init_temp(temperature);
    if temperature_sampler.is_null() {
        return Err(error::error!("Failed to create talker temperature sampler"));
    }
    unsafe {
        chain.chain_add(temperature_sampler);
    }
    // LLAMA_DEFAULT_SEED keeps unseeded runs non-reproducible, as llama.cpp does.
    let distribution_sampler = rust_de_llama::llama_sampler_init_dist(seed.unwrap_or(0xFFFFFFFF));
    if distribution_sampler.is_null() {
        return Err(error::error!(
            "Failed to create talker distribution sampler"
        ));
    }
    unsafe {
        chain.chain_add(distribution_sampler);
    }
    Ok(chain)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_normalize_text_separates_words() {
        assert_eq!(
            normalize_text("Hello, world!"),
            "hello<|text_sep|>world<|text_sep|>"
        );
    }

    #[test]
    fn test_normalize_text_splits_on_punctuation_and_drops_symbols() {
        // The number is spoken; only the '%' itself is dropped.
        assert_eq!(
            normalize_text("well-known 42% ok"),
            "well<|text_sep|>known<|text_sep|>forty<|text_sep|>two<|text_sep|>ok<|text_sep|>"
        );
    }

    #[test]
    fn test_normalize_text_without_letters_is_empty() {
        assert_eq!(normalize_text("!!"), "");
    }

    /// Digits reach the talker only if they are spelled first: the filter in
    /// `normalize_text` keeps letters only, so an unspelled number is not said
    /// at all rather than mis-said.
    #[test]
    fn test_spell_numbers_replaces_digit_runs() {
        assert_eq!(spell_numbers("42"), "forty-two");
        assert_eq!(spell_numbers("0"), "zero");
        assert_eq!(spell_numbers("7 of 19"), "seven of nineteen");
        assert_eq!(spell_numbers("in 2026!"), "in two thousand twenty-six!");
        assert_eq!(spell_numbers("105"), "one hundred five");
        assert_eq!(spell_numbers("no digits"), "no digits");
    }

    /// A digit run too large to spell must be dropped, not indexed with. The
    /// billions digit is what reaches `spell_below_thousand` unreduced, so a
    /// type wider than the tables allow panics rather than mis-says.
    #[test]
    fn test_spell_numbers_drops_runs_too_large_to_spell() {
        assert_eq!(spell_numbers("4294967295"), "four billion two hundred ninety-four million nine hundred sixty-seven thousand two hundred ninety-five");
        // Past u32: upstream's stoi throws here and yields a blank the same way.
        assert_eq!(spell_numbers("4294967296"), " ");
        assert_eq!(spell_numbers("2000000000000"), " ");
        assert_eq!(spell_numbers("18446744073709551616"), " ");
        // Still surrounded correctly rather than swallowing the sentence.
        assert_eq!(spell_numbers("a 99999999999999 b"), "a   b");
    }

    #[test]
    fn test_spell_numbers_handles_fractions_and_boundaries() {
        assert_eq!(spell_numbers("3.5"), "three point five");
        // A '.' only continues the run when a digit follows it.
        assert_eq!(spell_numbers("42. Next"), "forty-two. Next");
        assert_eq!(spell_numbers("1000000"), "one million ");
    }

    #[test]
    fn test_normalize_text_speaks_numbers() {
        assert_eq!(
            normalize_text("Answer: 42"),
            "answer<|text_sep|>forty<|text_sep|>two<|text_sep|>"
        );
        // Previously this normalized to "" and the reply could not be spoken.
        assert!(!normalize_text("42").is_empty());
    }

    /// Talker-native synthesis: with the text to speak supplied as tokens, a
    /// pretrained talker must drive the decoder to real speech. This exercises
    /// the AR loop, the end-of-generation stop and the code mapping against
    /// known-good weights, which leaves the projection as the pipeline's only
    /// untrained stage. Skipped when the GGUFs are absent (`models/` is
    /// gitignored); `e2e.sh` fetches them and checks the audio is intelligible.
    #[test]
    fn test_generate_native_speech() {
        let models = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("models");
        let talker_path = models.join("OuteTTS-0.2-500M-F16.gguf");
        let decoder_path = models.join("WavTokenizer-Large-75-F16.gguf");
        if !talker_path.exists() || !decoder_path.exists() {
            eprintln!("skipped: {} is absent", talker_path.display());
            return;
        }

        let text = "the quick brown fox jumps over the lazy dog";
        rust_de_llama::ensure_backend_init();
        let mut talker = Talker::load(&talker_path, 4096, 0).unwrap();

        let prefix_tokens = talker.prompt_prefix_tokens().to_vec();
        let suffix_tokens = talker.prompt_suffix_tokens().to_vec();
        let text_tokens = talker.prompt_text_tokens(text).unwrap();
        let output = talker
            .generate(
                &[
                    PrefixSegment::Tokens(&prefix_tokens),
                    PrefixSegment::Tokens(&text_tokens),
                    PrefixSegment::Tokens(&suffix_tokens),
                ],
                crate::audio::wavtokenizer::MAX_CODES,
                0.0,
                Some(42),
            )
            .unwrap();

        // Nine words are well inside one run, so the talker must have stopped
        // on its own rather than been cut off.
        assert!(output.complete, "talker did not reach end-of-generation");
        // 75 codes/second: nine words cannot be spoken in under a second, and
        // must not run to the code limit either.
        assert!(
            (75..crate::audio::wavtokenizer::MAX_CODES).contains(&output.codes.len()),
            "implausible code count {}",
            output.codes.len()
        );
        assert!(output
            .codes
            .iter()
            .all(|&code| (0..AUDIO_CODE_COUNT).contains(&code)));

        let mut decoder = crate::audio::wavtokenizer::WavTokenizer::load(&decoder_path, 0).unwrap();
        let samples = decoder.decode(&output.codes).unwrap();
        let rms = (samples.iter().map(|s| s * s).sum::<f32>() / samples.len() as f32).sqrt();
        assert!((0.005..=0.5).contains(&rms), "rms {rms}");

        let wav_path = std::env::temp_dir().join(format!(
            "rust_de_llama-talker-native-{}.wav",
            std::process::id()
        ));
        std::fs::write(
            &wav_path,
            crate::audio::pipeline::encode_wav(&samples, crate::audio::wavtokenizer::SAMPLE_RATE),
        )
        .unwrap();
        eprintln!(
            "talker-native '{}' -> {} codes -> {}",
            text,
            output.codes.len(),
            wav_path.display()
        );
    }

    /// A context too small for the speaker prompt must be refused at load, not
    /// once it is serving and every request fails.
    #[test]
    fn test_load_rejects_context_too_small_for_speaker_prompt() {
        let talker_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("models/OuteTTS-0.2-500M-F16.gguf");
        if !talker_path.exists() {
            eprintln!("skipped: {} is absent", talker_path.display());
            return;
        }

        rust_de_llama::ensure_backend_init();
        assert!(Talker::load(&talker_path, 1024, 0).is_err());
    }

    /// A prefix that fills the context must be refused rather than decoded past
    /// its end: codes are strictly fewer than the positions they occupy, so the
    /// code limit alone cannot keep the sequence in bounds.
    #[test]
    fn test_generate_rejects_prefix_that_fills_the_context() {
        let talker_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("models/OuteTTS-0.2-500M-F16.gguf");
        if !talker_path.exists() {
            eprintln!("skipped: {} is absent", talker_path.display());
            return;
        }

        rust_de_llama::ensure_backend_init();
        let n_ctx = 4096;
        let mut talker = Talker::load(&talker_path, n_ctx, 0).unwrap();
        let prefix = vec![0i32; n_ctx as usize];
        let result = talker.generate(
            &[PrefixSegment::Tokens(&prefix)],
            crate::audio::wavtokenizer::MAX_CODES,
            0.0,
            Some(42),
        );
        assert!(result.is_err());
    }

    /// The talker's budget for the text is what the pipeline validates input
    /// against; it must account for the speaker prompt and the codes.
    #[test]
    fn test_max_text_positions_excludes_speaker_prompt_and_codes() {
        let talker_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("models/OuteTTS-0.2-500M-F16.gguf");
        if !talker_path.exists() {
            eprintln!("skipped: {} is absent", talker_path.display());
            return;
        }

        rust_de_llama::ensure_backend_init();
        let n_ctx = 4096usize;
        let talker = Talker::load(&talker_path, n_ctx as i32, 0).unwrap();
        let scaffolding = talker.prompt_prefix_tokens().len() + talker.prompt_suffix_tokens().len();
        let max_codes = crate::audio::wavtokenizer::MAX_CODES;

        assert_eq!(
            talker.max_text_positions(max_codes),
            n_ctx - scaffolding - max_codes
        );
        // A code limit the context cannot seat leaves no room, rather than
        // underflowing into a huge budget.
        assert_eq!(talker.max_text_positions(n_ctx), 0);
    }

    /// The speaker example is the default voice; a truncated or reordered asset
    /// would degrade synthesis without failing any other test.
    #[test]
    fn test_speaker_assets_are_well_formed() {
        assert!(SPEAKER_TEXT.starts_with("<|text_start|>"));
        assert!(SPEAKER_TEXT.ends_with("<|text_sep|>"));
        assert!(SPEAKER_AUDIO.starts_with("<|audio_start|>\n"));
        assert!(SPEAKER_AUDIO.ends_with("<|code_end|>"));
        assert_eq!(
            SPEAKER_TEXT.matches("<|text_sep|>").count(),
            SPEAKER_AUDIO.matches("<|code_start|>").count(),
            "speaker text and audio disagree on word count"
        );
    }
}
