//! WavTokenizer decoder: audio-codec tokens -> waveform.
//!
//! The model is loaded through llama.cpp's native `wavtokenizer-dec`
//! architecture: codes are fed as tokens to an embeddings context via
//! `llama_encode`, producing a `[n_codes, n_embd]` log-magnitude/phase
//! spectrogram; the inverse STFT (`embd_to_audio`, ported from llama.cpp
//! `tools/tts/tts.cpp`) folds it into 24 kHz mono samples.

pub const SAMPLE_RATE: u32 = 24000;
/// STFT geometry of wavtokenizer-large-75 (75 codes/second at 24 kHz).
const N_FFT: usize = 1280;
const N_HOP: usize = 320;
/// Upper bound on codes per decode; sizes the context and batch (~27 s).
pub const MAX_CODES: usize = 2048;

pub struct WavTokenizer {
    model: rust_de_llama::LlamaModel,
    context: rust_de_llama::LlamaContext,
    batch_buffer: crate::parallel::batch_buffer::BatchBuffer,
    n_embd: usize,
}

impl WavTokenizer {
    #[tracing::instrument]
    pub fn load(path: &std::path::Path, n_threads: i32) -> Result<Self, error::Error> {
        let model =
            rust_de_llama::LlamaModel::load_from_file(path, None, None, false).map_err(|e| {
                error::error!("Failed to load audio decoder '{}': {}", path.display(), e)
            })?;
        let n_embd = model.n_embd();
        // embd_to_audio consumes n_fft/2 + 1 (magnitude, phase) bin pairs per
        // frame, so the decoder must emit exactly N_FFT + 2 channels; llama.cpp
        // strides the embeddings buffer by n_embd_out, so that must match too.
        if n_embd != (N_FFT + 2) as i32 || model.n_embd_out() != n_embd {
            return Err(error::error!(
                "Audio decoder '{}' has n_embd {} (n_embd_out {}), expected {} (wavtokenizer-large STFT geometry)",
                path.display(),
                n_embd,
                model.n_embd_out(),
                N_FFT + 2
            ));
        }
        // The vocoder consumes the whole code sequence in one shot, so the
        // batch and micro-batch must both hold MAX_CODES tokens.
        let context = rust_de_llama::LlamaContext::new_with_embeddings(
            &model,
            MAX_CODES as i32,
            MAX_CODES as i32,
            MAX_CODES as i32,
            n_threads,
            n_threads,
        )
        .map_err(|e| error::error!("Failed to create audio decoder context: {}", e))?;

        tracing::info!(
            "Loaded audio decoder '{}': n_embd={}, vocab={}",
            path.display(),
            n_embd,
            model.n_vocab()
        );

        Ok(Self {
            model,
            context,
            batch_buffer: crate::parallel::batch_buffer::BatchBuffer::new(MAX_CODES),
            n_embd: n_embd as usize,
        })
    }

    pub fn vocab_size(&self) -> i32 {
        self.model.n_vocab()
    }

    #[tracing::instrument(skip(self, codes))]
    pub fn decode(&mut self, codes: &[i32]) -> Result<Vec<f32>, error::Error> {
        if codes.is_empty() {
            return Err(error::error!("Cannot decode an empty code sequence"));
        }
        if codes.len() > MAX_CODES {
            return Err(error::error!(
                "Too many audio codes: {} exceeds the limit of {}",
                codes.len(),
                MAX_CODES
            ));
        }
        let vocab_size = self.vocab_size();
        if let Some(&code) = codes.iter().find(|&&code| code < 0 || code >= vocab_size) {
            return Err(error::error!(
                "Audio code {} is outside the decoder vocabulary (0..{})",
                code,
                vocab_size
            ));
        }

        self.batch_buffer.reset();
        for (position, &code) in codes.iter().enumerate() {
            self.batch_buffer.add_token(code, position as i32, 0, 1);
        }
        let batch = self.batch_buffer.as_llama_batch();
        self.context
            .encode(batch)
            .map_err(|e| error::error!("Audio decoder encode failed: {}", e))?;
        self.context.synchronize();

        let spectrogram = self
            .context
            .embeddings(codes.len(), self.n_embd)
            .map_err(|e| error::error!("Failed to read decoder embeddings: {}", e))?;

        Ok(embd_to_audio(&spectrogram, codes.len(), self.n_embd))
    }
}

/// Periodic Hann window, as in llama.cpp `fill_hann_window`.
fn hann_window(length: usize) -> Vec<f32> {
    (0..length)
        .map(|i| 0.5 * (1.0 - ((2.0 * std::f32::consts::PI * i as f32) / length as f32).cos()))
        .collect()
}

/// Inverse real FFT by direct evaluation, as in llama.cpp `irfft`:
/// `complex_input` holds `n / 2 + 1` interleaved (re, im) coefficients.
/// Faithful port: the reference normalizes by `n / 2 + 1` (not `n`) and does
/// not halve the DC/Nyquist bins; parity with the decoder llama.cpp ships for
/// this model is preferred over textbook iSTFT (the envelope normalization
/// in `embd_to_audio` cancels constant gain anyway).
fn irfft(n: usize, complex_input: &[f32], output: &mut [f32]) {
    let bins = n / 2 + 1;
    for (k, value) in output.iter_mut().enumerate().take(n) {
        let mut sum = 0.0f32;
        for m in 0..bins {
            let angle = 2.0 * std::f32::consts::PI * (k * m) as f32 / n as f32;
            sum += complex_input[2 * m] * angle.cos() - complex_input[2 * m + 1] * angle.sin();
        }
        *value = sum / bins as f32;
    }
}

/// Overlap-add fold, as in llama.cpp `fold` (torch.nn.functional.fold on a
/// [n_frames, n_win] column matrix with the padding trimmed).
fn fold(data: &[f32], n_out: usize, n_win: usize, n_hop: usize, n_pad: usize) -> Vec<f32> {
    let mut output = vec![0.0f32; n_out];
    let mut column_index = 0usize;
    let frames = data.len() / n_win;
    for frame in 0..frames {
        let start = (frame * n_hop) as i64 - n_pad as i64;
        for offset in 0..n_win as i64 {
            let position = start + offset;
            if (0..n_out as i64).contains(&position) {
                output[position as usize] += data[column_index];
            }
            column_index += 1;
        }
    }
    output.truncate(n_out - 2 * n_pad);
    output
}

/// Spectrogram embeddings -> waveform, ported from llama.cpp `embd_to_audio`:
/// the first `n_embd / 2` channels are log-magnitude (clamped at 1e2 after
/// exp) and the rest phase; each frame is inverse-FFT'd, Hann-windowed, and
/// overlap-added with envelope normalization.
fn embd_to_audio(embd: &[f32], n_codes: usize, n_embd: usize) -> Vec<f32> {
    let n_win = N_FFT;
    let n_pad = (n_win - N_HOP) / 2;
    let n_out = (n_codes - 1) * N_HOP + n_win;
    let hann = hann_window(N_FFT);

    // Interleaved (re, im) spectrum per frame, frame-major.
    let mut spectrum = vec![0.0f32; n_codes * n_embd];
    for frame in 0..n_codes {
        for bin in 0..n_embd / 2 {
            let magnitude = embd[frame * n_embd + bin].exp().min(1e2);
            let phase = embd[frame * n_embd + bin + n_embd / 2];
            spectrum[frame * n_embd + 2 * bin] = magnitude * phase.cos();
            spectrum[frame * n_embd + 2 * bin + 1] = magnitude * phase.sin();
        }
    }

    // Frame index with its output windows, sharded round-robin per worker.
    type FrameTask<'a> = (usize, &'a mut [f32], &'a mut [f32]);

    let mut frames = vec![0.0f32; n_codes * N_FFT];
    let mut envelope = vec![0.0f32; n_codes * N_FFT];
    let worker_count = std::thread::available_parallelism()
        .map(|n| n.get())
        .unwrap_or(4)
        .min(n_codes.max(1));
    std::thread::scope(|scope| {
        let frame_chunks = frames.chunks_mut(N_FFT).collect::<Vec<_>>();
        let envelope_chunks = envelope.chunks_mut(N_FFT).collect::<Vec<_>>();
        let mut work: Vec<Vec<FrameTask>> = (0..worker_count).map(|_| Vec::new()).collect();
        for (index, (frame, env)) in frame_chunks
            .into_iter()
            .zip(envelope_chunks.into_iter())
            .enumerate()
        {
            work[index % worker_count].push((index, frame, env));
        }
        for chunk in work {
            let spectrum = &spectrum;
            let hann = &hann;
            scope.spawn(move || {
                for (index, frame, env) in chunk {
                    irfft(
                        N_FFT,
                        &spectrum[index * n_embd..(index + 1) * n_embd],
                        frame,
                    );
                    for j in 0..N_FFT {
                        frame[j] *= hann[j];
                        env[j] = hann[j] * hann[j];
                    }
                }
            });
        }
    });

    let audio = fold(&frames, n_out, n_win, N_HOP, n_pad);
    let envelope = fold(&envelope, n_out, n_win, N_HOP, n_pad);
    audio
        .iter()
        .zip(&envelope)
        .map(|(sample, env)| if *env > 0.0 { sample / env } else { 0.0 })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hann_window_endpoints_and_midpoint() {
        let window = hann_window(8);
        assert!(window[0].abs() < 1e-6);
        assert!((window[4] - 1.0).abs() < 1e-6);
        assert!((window[2] - 0.5).abs() < 1e-6);
    }

    #[test]
    fn test_irfft_dc_component() {
        // Spectrum with only the DC bin set: output is constant N/N = 1 per
        // the reference implementation's 1/N normalization (N = n/2 + 1).
        let n = 8;
        let bins = n / 2 + 1;
        let mut spectrum = vec![0.0f32; bins * 2];
        spectrum[0] = bins as f32;
        let mut output = vec![0.0f32; n];
        irfft(n, &spectrum, &mut output);
        for value in output {
            assert!((value - 1.0).abs() < 1e-5);
        }
    }

    #[test]
    fn test_fold_overlap_add() {
        // Two frames of ones, n_win 4, n_hop 2, no padding: positions 2..4
        // overlap and sum to 2.
        let data = vec![1.0f32; 8];
        let output = fold(&data, 6, 4, 2, 0);
        assert_eq!(output, Vec::from([1.0, 1.0, 2.0, 2.0, 1.0, 1.0]));
    }

    #[test]
    fn test_embd_to_audio_output_length() {
        let n_codes = 3;
        let n_embd = N_FFT + 2;
        let embd = vec![0.0f32; n_codes * n_embd];
        let audio = embd_to_audio(&embd, n_codes, n_embd);
        let n_win = N_FFT;
        let n_pad = (n_win - N_HOP) / 2;
        assert_eq!(audio.len(), (n_codes - 1) * N_HOP + n_win - 2 * n_pad);
    }

    /// End-to-end decode of real codes for a known utterance, taken from the
    /// reference prompt in llama.cpp `tools/tts/tts.cpp`. The window/fold unit
    /// tests above cover the iSTFT piecewise on synthetic input; only real
    /// codes catch a decoder or iSTFT regression that still produces
    /// plausibly-shaped output. Skipped when the decoder GGUF is absent
    /// (`models/` is gitignored); `e2e.sh` fetches it.
    #[test]
    fn test_decode_golden_codes() {
        let model_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("models/WavTokenizer-Large-75-F16.gguf");
        if !model_path.exists() {
            eprintln!("skipped: {} is absent", model_path.display());
            return;
        }

        let codes: Vec<i32> = include_str!("../../tests/fixtures/wavtokenizer_golden_codes.txt")
            .split_whitespace()
            .map(|line| line.parse().unwrap())
            .collect();

        rust_de_llama::ensure_backend_init();
        let mut decoder = WavTokenizer::load(&model_path, 0).unwrap();
        let samples = decoder.decode(&codes).unwrap();

        // 75 codes/second at 24 kHz, minus the iSTFT's trimmed padding.
        let n_pad = (N_FFT - N_HOP) / 2;
        assert_eq!(samples.len(), (codes.len() - 1) * N_HOP + N_FFT - 2 * n_pad);

        // Speech, not silence and not a saturated buzz: the reference
        // utterance sits well inside the representable range.
        let peak = samples.iter().fold(0.0f32, |peak, s| peak.max(s.abs()));
        let rms = (samples.iter().map(|s| s * s).sum::<f32>() / samples.len() as f32).sqrt();
        assert!((0.05..=1.0).contains(&peak), "peak {peak}");
        assert!((0.005..=0.5).contains(&rms), "rms {rms}");

        let wav_path =
            std::env::temp_dir().join(format!("rust_de_llama-golden-{}.wav", std::process::id()));
        std::fs::write(
            &wav_path,
            crate::audio::pipeline::encode_wav(&samples, SAMPLE_RATE),
        )
        .unwrap();
        eprintln!("golden decode written to {}", wav_path.display());
    }
}
