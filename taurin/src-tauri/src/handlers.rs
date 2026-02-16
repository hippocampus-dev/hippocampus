use tauri::{Emitter, Manager};

pub fn hot_key_handler<R>(
    value: tauri_plugin_global_shortcut::Shortcut,
) -> Box<
    dyn Fn(
            &tauri::AppHandle<R>,
            &tauri_plugin_global_shortcut::Shortcut,
            tauri_plugin_global_shortcut::ShortcutEvent,
        ) + Send
        + Sync
        + 'static,
>
where
    R: tauri::Runtime,
{
    Box::new(move |app, shortcut, event| {
        let window = app.get_webview_window("main").unwrap();
        if shortcut == &value {
            match event.state {
                tauri_plugin_global_shortcut::ShortcutState::Pressed => {
                    if window.is_visible().unwrap() {
                        window.hide().unwrap();
                    } else {
                        window.show().unwrap();
                        window.set_focus().unwrap();
                    }
                }
                _ => {}
            }
        }
    })
}

pub fn voice_input_handler<R>(
    value: tauri_plugin_global_shortcut::Shortcut,
) -> Box<
    dyn Fn(
            &tauri::AppHandle<R>,
            &tauri_plugin_global_shortcut::Shortcut,
            tauri_plugin_global_shortcut::ShortcutEvent,
        ) + Send
        + Sync
        + 'static,
>
where
    R: tauri::Runtime,
{
    Box::new(move |app, shortcut, event| {
        if shortcut != &value {
            return;
        }

        let app = app.clone();
        match event.state {
            tauri_plugin_global_shortcut::ShortcutState::Pressed => {
                tauri::async_runtime::spawn(async move {
                    let (recording, capture_stop, audio_buffer, device_name, capture_notify) = {
                        let state = app.state::<tokio::sync::Mutex<crate::AppState>>();
                        let state = state.lock().await;

                        if state
                            .voice_input
                            .recording
                            .load(std::sync::atomic::Ordering::Relaxed)
                        {
                            return;
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
                        if let Err(e) = crate::commands::voice_input::internal::capture_microphone(
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
                });
            }
            tauri_plugin_global_shortcut::ShortcutState::Released => {
                tauri::async_runtime::spawn(async move {
                    let (
                        recording,
                        capture_stop,
                        audio_buffer,
                        model,
                        model_name,
                        language,
                        capture_notify,
                    ) = {
                        let state = app.state::<tokio::sync::Mutex<crate::AppState>>();
                        let state = state.lock().await;

                        (
                            std::sync::Arc::clone(&state.voice_input.recording),
                            std::sync::Arc::clone(&state.voice_input.capture_stop),
                            std::sync::Arc::clone(&state.voice_input.audio_buffer),
                            std::sync::Arc::clone(&state.voice_input.model),
                            state.voice_input.model_name.clone(),
                            state.voice_input.language.clone(),
                            std::sync::Arc::clone(&state.voice_input.capture_notify),
                        )
                    };

                    recording.store(false, std::sync::atomic::Ordering::Relaxed);
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
                        return;
                    }

                    {
                        let mut model_guard = model.lock().await;
                        if model_guard.is_none() {
                            match crate::commands::voice_input::internal::get_model_path(
                                &app,
                                &model_name,
                            ) {
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
                                            return;
                                        }
                                    }
                                }
                                _ => {
                                    eprintln!("Whisper model not downloaded yet");
                                    let _ = app.emit("voice-input-state", "idle");
                                    if let Some(indicator) =
                                        app.get_webview_window("voice-indicator")
                                    {
                                        let _ = indicator.hide();
                                    }
                                    return;
                                }
                            }
                        }
                    }

                    let cloned_model = std::sync::Arc::clone(&model);
                    let result = tokio::task::spawn_blocking(move || {
                        let guard = cloned_model.blocking_lock();
                        if let Some(context) = guard.as_ref() {
                            crate::commands::voice_input::internal::transcribe(
                                context, &samples, &language,
                            )
                        } else {
                            Err("model not loaded".into())
                        }
                    })
                    .await;

                    match result {
                        Ok(Ok(text)) if !text.is_empty() => {
                            let _ = tokio::task::spawn_blocking(move || {
                                if let Err(e) =
                                    crate::commands::voice_input::internal::simulate_keyboard_input(
                                        &text,
                                    )
                                {
                                    eprintln!(
                                        "Error occurred while simulating keyboard input: {e}"
                                    );
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
                });
            }
        }
    })
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
