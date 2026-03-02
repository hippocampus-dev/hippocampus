const WHISPER_SAMPLE_RATE: u32 = 16000;

#[tauri::command]
pub fn list_audio_input_devices() -> Result<Vec<String>, String> {
    use cpal::traits::{DeviceTrait, HostTrait};

    let host = cpal::default_host();
    let devices = host
        .input_devices()
        .map_err(|e| format!("failed to enumerate input devices: {e}"))?;

    let names: Vec<String> = devices.filter_map(|device| device.name().ok()).collect();

    Ok(names)
}

#[tauri::command]
pub async fn download_whisper_model(
    app_handle: tauri::AppHandle,
    model_name: String,
    progress_tx: tauri::ipc::Channel<f64>,
) -> Result<(), String> {
    internal::download_model(&app_handle, &model_name, &progress_tx)
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn get_whisper_model_status(
    app_handle: tauri::AppHandle,
    model_name: String,
) -> Result<bool, String> {
    let path = internal::get_model_path(&app_handle, &model_name).map_err(|e| e.to_string())?;
    Ok(path.exists())
}

#[tauri::command]
pub async fn start_voice_input(
    app_handle: tauri::AppHandle,
    _state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<(), String> {
    internal::start_recording(app_handle)
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn stop_voice_input(
    app_handle: tauri::AppHandle,
    _state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<(), String> {
    internal::stop_recording_and_transcribe(app_handle)
        .await
        .map_err(|e| e.to_string())
}

pub(crate) mod internal {
    use super::*;

    pub(crate) fn get_model_path<R: tauri::Runtime>(
        app_handle: &tauri::AppHandle<R>,
        model_name: &str,
    ) -> Result<std::path::PathBuf, Box<dyn std::error::Error + Send + Sync + 'static>> {
        use tauri::Manager;
        let app_data_dir = app_handle.path().app_data_dir()?;
        Ok(app_data_dir
            .join("models")
            .join(format!("ggml-{model_name}.bin")))
    }

    pub(crate) async fn download_model<R: tauri::Runtime>(
        app_handle: &tauri::AppHandle<R>,
        model_name: &str,
        progress_tx: &tauri::ipc::Channel<f64>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let model_path = get_model_path(app_handle, model_name)?;

        if model_path.exists() {
            let _ = progress_tx.send(1.0);
            return Ok(());
        }

        if let Some(parent) = model_path.parent() {
            std::fs::create_dir_all(parent)?;
        }

        let url = format!(
            "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-{model_name}.bin"
        );
        let uri: hyper::Uri = url.parse()?;

        let https = {
            use hyper_rustls::ConfigBuilderExt;
            let config = rustls::ClientConfig::builder()
                .with_safe_defaults()
                .with_native_roots()
                .with_no_client_auth();
            hyper_rustls::HttpsConnectorBuilder::new()
                .with_tls_config(config)
                .https_or_http()
                .enable_http1()
                .build()
        };
        let client: hyper::Client<_, hyper::Body> = hyper::Client::builder().build(https);

        let response = client.get(uri.clone()).await?;

        if !response.status().is_success() {
            if response.status() == hyper::StatusCode::MOVED_PERMANENTLY
                || response.status() == hyper::StatusCode::FOUND
                || response.status() == hyper::StatusCode::TEMPORARY_REDIRECT
            {
                if let Some(location) = response.headers().get(hyper::header::LOCATION) {
                    let redirect_uri: hyper::Uri = location.to_str()?.parse()?;
                    let response = client.get(redirect_uri).await?;
                    if !response.status().is_success() {
                        return Err(format!(
                            "failed to download model (redirect): status {}",
                            response.status()
                        )
                        .into());
                    }
                    return write_response_to_file(response, &model_path, progress_tx).await;
                }
            }
            return Err(format!("failed to download model: status {}", response.status()).into());
        }

        write_response_to_file(response, &model_path, progress_tx).await
    }

    async fn write_response_to_file(
        response: hyper::Response<hyper::Body>,
        model_path: &std::path::Path,
        progress_tx: &tauri::ipc::Channel<f64>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        use futures_util::StreamExt;

        let content_length = response
            .headers()
            .get(hyper::header::CONTENT_LENGTH)
            .and_then(|v| v.to_str().ok())
            .and_then(|v| v.parse::<u64>().ok())
            .unwrap_or(0);

        let tmp_path = model_path.with_extension("bin.tmp");
        let mut file = std::fs::File::create(&tmp_path)?;
        let mut downloaded: u64 = 0;
        let mut body = response.into_body();

        while let Some(chunk) = body.next().await {
            let chunk = chunk?;
            std::io::Write::write_all(&mut file, &chunk)?;
            downloaded += chunk.len() as u64;

            if content_length > 0 {
                let progress = downloaded as f64 / content_length as f64;
                let _ = progress_tx.send(progress);
            }
        }

        drop(file);
        std::fs::rename(&tmp_path, model_path)?;
        let _ = progress_tx.send(1.0);

        Ok(())
    }

    pub(crate) fn capture_microphone(
        running: std::sync::Arc<std::sync::atomic::AtomicBool>,
        capture_stop: std::sync::Arc<(std::sync::Mutex<bool>, std::sync::Condvar)>,
        audio_buffer: std::sync::Arc<std::sync::Mutex<Vec<f32>>>,
        device_name: Option<String>,
        mic_ready: std::sync::Arc<tokio::sync::Notify>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        use cpal::traits::{DeviceTrait, HostTrait, StreamTrait};

        let host = cpal::default_host();
        let device = if let Some(name) = device_name {
            host.input_devices()?
                .find(|d| d.name().map(|n| n == name).unwrap_or(false))
                .ok_or_else(|| format!("input device not found: {name}"))?
        } else {
            host.default_input_device()
                .ok_or("no default input device")?
        };

        let config = device.default_input_config()?;
        let sample_rate = config.sample_rate().0;
        let channels = config.channels() as u32;

        let cloned_running = std::sync::Arc::clone(&running);
        let cloned_buffer = std::sync::Arc::clone(&audio_buffer);
        let notified = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));

        let cloned_notified = std::sync::Arc::clone(&notified);
        let cloned_mic_ready = std::sync::Arc::clone(&mic_ready);
        let stream = match config.sample_format() {
            cpal::SampleFormat::F32 => device.build_input_stream(
                &config.into(),
                move |data: &[f32], _: &cpal::InputCallbackInfo| {
                    if !cloned_running.load(std::sync::atomic::Ordering::Relaxed) {
                        return;
                    }
                    if !cloned_notified.load(std::sync::atomic::Ordering::Relaxed) {
                        cloned_notified.store(true, std::sync::atomic::Ordering::Relaxed);
                        cloned_mic_ready.notify_one();
                    }
                    let mono: Vec<f32> = if channels > 1 {
                        data.chunks(channels as usize)
                            .map(|chunk| chunk.iter().sum::<f32>() / chunk.len() as f32)
                            .collect()
                    } else {
                        data.to_vec()
                    };
                    if let Some(mut buffer) = cloned_buffer.lock().ok() {
                        buffer.extend_from_slice(&mono);
                    }
                },
                |e| eprintln!("input stream error: {e}"),
                None,
            )?,
            cpal::SampleFormat::I16 => {
                let cloned_running = std::sync::Arc::clone(&running);
                let cloned_buffer = std::sync::Arc::clone(&audio_buffer);
                let cloned_notified = std::sync::Arc::clone(&notified);
                let cloned_mic_ready = std::sync::Arc::clone(&mic_ready);
                device.build_input_stream(
                    &config.into(),
                    move |data: &[i16], _: &cpal::InputCallbackInfo| {
                        if !cloned_running.load(std::sync::atomic::Ordering::Relaxed) {
                            return;
                        }
                        if !cloned_notified.load(std::sync::atomic::Ordering::Relaxed) {
                            cloned_notified.store(true, std::sync::atomic::Ordering::Relaxed);
                            cloned_mic_ready.notify_one();
                        }
                        let mono: Vec<f32> = if channels > 1 {
                            data.chunks(channels as usize)
                                .map(|chunk| {
                                    let sum: f32 = chunk.iter().map(|&s| s as f32 / 32768.0).sum();
                                    sum / chunk.len() as f32
                                })
                                .collect()
                        } else {
                            data.iter().map(|&s| s as f32 / 32768.0).collect()
                        };
                        if let Some(mut buffer) = cloned_buffer.lock().ok() {
                            buffer.extend_from_slice(&mono);
                        }
                    },
                    |e| eprintln!("input stream error: {e}"),
                    None,
                )?
            }
            format => {
                return Err(format!("unsupported sample format: {format:?}").into());
            }
        };

        stream.play()?;

        {
            let guard = capture_stop.0.lock().unwrap();
            drop(capture_stop.1.wait_while(guard, |stopped| !*stopped));
        }

        drop(stream);

        if sample_rate != WHISPER_SAMPLE_RATE {
            let mut buffer = audio_buffer.lock().unwrap();
            let resampled = resample_f32(&buffer, sample_rate, WHISPER_SAMPLE_RATE);
            buffer.clear();
            buffer.extend_from_slice(&resampled);
        }

        Ok(())
    }

    pub(crate) fn transcribe(
        context: &whisper_rs::WhisperContext,
        samples: &[f32],
        language: &str,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let mut state = context.create_state()?;

        let mut params =
            whisper_rs::FullParams::new(whisper_rs::SamplingStrategy::Greedy { best_of: 1 });
        params.set_language(Some(language));
        params.set_print_special(false);
        params.set_print_progress(false);
        params.set_print_realtime(false);
        params.set_print_timestamps(false);
        params.set_suppress_blank(true);

        state.full(params, samples)?;

        let num_segments = state.full_n_segments();
        let mut text = String::new();
        for i in 0..num_segments {
            if let Some(segment) = state.get_segment(i) {
                if let Ok(s) = segment.to_str_lossy() {
                    text.push_str(&s);
                }
            }
        }

        let text = text.trim().to_string();
        if is_likely_hallucination(&text) {
            return Ok(String::new());
        }
        Ok(text)
    }

    fn is_likely_hallucination(text: &str) -> bool {
        if text.chars().count() <= 2 {
            return true;
        }
        if text.chars().all(|c| {
            c.is_ascii_punctuation() || c.is_whitespace() || "。、！？「」『』（）…─　".contains(c)
        }) {
            return true;
        }
        let lower = text.to_lowercase();
        [
            "thank you",
            "thanks for watching",
            "bye",
            "the end",
            "subtitle",
            "ご視聴ありがとうございました",
        ]
        .iter()
        .any(|pattern| {
            lower == *pattern || (lower.contains(pattern) && lower.len() < pattern.len() + 10)
        })
    }

    pub(crate) fn simulate_keyboard_input(
        text: &str,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        use enigo::{Enigo, Keyboard, Settings};

        let mut enigo = Enigo::new(&Settings::default())?;
        enigo.text(text)?;
        Ok(())
    }

    pub(crate) async fn start_recording<R: tauri::Runtime>(
        app: tauri::AppHandle<R>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        use tauri::{Emitter, Manager};

        let (recording, capture_stop, audio_buffer, device_name, capture_notify) = {
            let state = app.state::<tokio::sync::Mutex<crate::AppState>>();
            let state = state.lock().await;

            if state
                .voice_input
                .recording
                .load(std::sync::atomic::Ordering::Relaxed)
            {
                return Ok(());
            }

            state
                .voice_input
                .recording
                .store(true, std::sync::atomic::Ordering::Relaxed);
            {
                let mut stopped = state.voice_input.capture_stop.0.lock().unwrap();
                *stopped = false;
            }
            {
                let mut buffer = state.voice_input.audio_buffer.lock().unwrap();
                buffer.clear();
            }

            (
                std::sync::Arc::clone(&state.voice_input.recording),
                std::sync::Arc::clone(&state.voice_input.capture_stop),
                std::sync::Arc::clone(&state.voice_input.audio_buffer),
                state.voice_input.device_name.clone(),
                std::sync::Arc::clone(&state.voice_input.capture_notify),
            )
        };

        let cursor_position = tokio::task::spawn_blocking(|| {
            use enigo::{Enigo, Mouse, Settings};
            Enigo::new(&Settings::default())
                .ok()
                .and_then(|enigo| enigo.location().ok())
        })
        .await
        .ok()
        .flatten();

        let _ = app.emit("voice-input-state", "starting");

        if let Some(indicator) = app.get_webview_window("voice-indicator") {
            let monitor = indicator
                .current_monitor()
                .ok()
                .flatten()
                .or_else(|| indicator.primary_monitor().ok().flatten());

            if let Some(monitor) = monitor {
                let scale = monitor.scale_factor();
                let screen_width = monitor.size().width as f64 / scale;
                let screen_height = monitor.size().height as f64 / scale;
                let margin = 16.0;
                let indicator_width = 200.0;
                let indicator_height = 56.0;

                let (x, y) = if let Some((cursor_x, cursor_y)) = cursor_position {
                    let cx = cursor_x as f64 / scale;
                    let cy = cursor_y as f64 / scale;
                    let x = cx.min(screen_width - indicator_width - margin).max(margin);
                    let y = if cy + 32.0 + indicator_height > screen_height - margin {
                        cy - indicator_height - 8.0
                    } else {
                        cy + 32.0
                    }
                    .max(margin);
                    (x, y)
                } else {
                    (
                        screen_width - indicator_width - 64.0,
                        screen_height - indicator_height - 64.0,
                    )
                };

                let _ = indicator.set_position(tauri::LogicalPosition::new(x, y));
            }
            let _ = indicator.show();
        }

        let mic_ready = std::sync::Arc::new(tokio::sync::Notify::new());
        let mic_ready_clone = std::sync::Arc::clone(&mic_ready);
        let app_clone = app.clone();
        tokio::spawn(async move {
            mic_ready_clone.notified().await;
            let _ = app_clone.emit("voice-input-state", "recording");
        });

        tokio::task::spawn_blocking(move || {
            if let Err(e) = capture_microphone(
                recording,
                capture_stop,
                audio_buffer,
                device_name,
                mic_ready,
            ) {
                eprintln!("Error occurred while capturing microphone: {e}");
            }
            capture_notify.notify_one();
        });

        Ok(())
    }

    pub(crate) async fn stop_recording_and_transcribe<R: tauri::Runtime>(
        app: tauri::AppHandle<R>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        use tauri::{Emitter, Manager};

        let (capture_stop, audio_buffer, model, model_name, language, capture_notify) = {
            let state = app.state::<tokio::sync::Mutex<crate::AppState>>();
            let state = state.lock().await;

            if !state
                .voice_input
                .recording
                .swap(false, std::sync::atomic::Ordering::Relaxed)
            {
                return Ok(());
            }

            (
                std::sync::Arc::clone(&state.voice_input.capture_stop),
                std::sync::Arc::clone(&state.voice_input.audio_buffer),
                std::sync::Arc::clone(&state.voice_input.model),
                state.voice_input.model_name.clone(),
                state.voice_input.language.clone(),
                std::sync::Arc::clone(&state.voice_input.capture_notify),
            )
        };

        {
            let mut stopped = capture_stop.0.lock().unwrap();
            *stopped = true;
            capture_stop.1.notify_one();
        }

        let _ = app.emit("voice-input-state", "processing");

        capture_notify.notified().await;

        let samples = {
            let buffer = audio_buffer.lock().unwrap();
            buffer.clone()
        };

        if samples.is_empty() || is_audio_silent(&samples) {
            let _ = app.emit("voice-input-state", "idle");
            if let Some(indicator) = app.get_webview_window("voice-indicator") {
                let _ = indicator.hide();
            }
            return Ok(());
        }

        {
            let mut model_guard = model.lock().await;
            if model_guard.is_none() {
                match get_model_path(&app, &model_name) {
                    Ok(path) if path.exists() => {
                        match whisper_rs::WhisperContext::new_with_params(
                            path.to_str().unwrap_or_default(),
                            whisper_rs::WhisperContextParameters::default(),
                        ) {
                            Ok(ctx) => {
                                *model_guard = Some(ctx);
                            }
                            Err(e) => {
                                eprintln!("Failed to load whisper model: {e}");
                                let _ = app.emit("voice-input-state", "idle");
                                if let Some(indicator) =
                                    app.get_webview_window("voice-indicator")
                                {
                                    let _ = indicator.hide();
                                }
                                return Ok(());
                            }
                        }
                    }
                    _ => {
                        eprintln!("Whisper model not downloaded yet");
                        let _ = app.emit("voice-input-state", "idle");
                        if let Some(indicator) = app.get_webview_window("voice-indicator") {
                            let _ = indicator.hide();
                        }
                        return Ok(());
                    }
                }
            }
        }

        let cloned_model = std::sync::Arc::clone(&model);
        let result = tokio::task::spawn_blocking(move || {
            let guard = cloned_model.blocking_lock();
            if let Some(context) = guard.as_ref() {
                transcribe(context, &samples, &language)
            } else {
                Err("model not loaded".into())
            }
        })
        .await;

        match result {
            Ok(Ok(text)) if !text.is_empty() => {
                let _ = tokio::task::spawn_blocking(move || {
                    if let Err(e) = simulate_keyboard_input(&text) {
                        eprintln!("Error occurred while simulating keyboard input: {e}");
                    }
                })
                .await;
            }
            Ok(Err(e)) => {
                eprintln!("Transcription error: {e}");
            }
            Err(e) => {
                eprintln!("Transcription task error: {e}");
            }
            _ => {}
        }

        let _ = app.emit("voice-input-state", "idle");
        if let Some(indicator) = app.get_webview_window("voice-indicator") {
            let _ = indicator.hide();
        }

        Ok(())
    }

    fn is_audio_silent(samples: &[f32]) -> bool {
        if samples.is_empty() {
            return true;
        }
        let rms = (samples
            .iter()
            .map(|&s| (s as f64) * (s as f64))
            .sum::<f64>()
            / samples.len() as f64)
            .sqrt();
        rms < 0.01
    }

    fn resample_f32(samples: &[f32], source_rate: u32, destination_rate: u32) -> Vec<f32> {
        if source_rate == destination_rate {
            return samples.to_vec();
        }

        if source_rate > destination_rate {
            let output_size =
                (samples.len() as f64 * destination_rate as f64 / source_rate as f64) as usize;
            let mut result = Vec::with_capacity(output_size);

            let samples_per_output = (source_rate as f64 / destination_rate as f64).ceil() as usize;
            let mut input_index = 0;

            while input_index < samples.len() {
                let end_index = std::cmp::min(input_index + samples_per_output, samples.len());

                if input_index < end_index {
                    let sum: f32 = samples[input_index..end_index].iter().sum();
                    let count = (end_index - input_index) as f32;
                    result.push(sum / count);
                }

                input_index += (source_rate as f64 / destination_rate as f64) as usize;
            }

            result
        } else {
            let output_size =
                (samples.len() as f64 * destination_rate as f64 / source_rate as f64) as usize;
            let mut result = Vec::with_capacity(output_size);

            let ratio = source_rate as f64 / destination_rate as f64;
            let input_size = samples.len();

            for output_index in 0..output_size {
                let source_index_f = output_index as f64 * ratio;
                let source_index = source_index_f as usize;

                if source_index + 1 < input_size {
                    let fraction = source_index_f - source_index as f64;
                    let interpolated = samples[source_index] as f64 * (1.0 - fraction)
                        + samples[source_index + 1] as f64 * fraction;
                    result.push(interpolated as f32);
                } else if source_index < input_size {
                    result.push(samples[source_index]);
                }
            }

            result
        }
    }
}
