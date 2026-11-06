const OPENAI_REALTIME_API_SAMPLING_RATE: u32 = 24000;
const OPENAI_REALTIME_API_CHANNELS: u8 = 1;
const OPENAI_REALTIME_API_MODEL: &str = "gpt-realtime-mini";

#[tauri::command]
pub async fn start_realtime_translation(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
    result_tx: tauri::ipc::Channel<String>,
) -> Result<(), ipc::CommandError> {
    let mut state = state.lock().await;
    if state
        .realtime_translation
        .volume_toggle
        .load(std::sync::atomic::Ordering::Relaxed)
    {
        return Err("already running".to_string().into());
    }
    // A task from a previous run can outlive its stop, so give this run its own flag rather than resetting the shared one
    state.realtime_translation.volume_toggle =
        std::sync::Arc::new(std::sync::atomic::AtomicBool::new(true));

    let (tx, rx) = tokio::sync::mpsc::channel::<Vec<u8>>(10);

    let cancelled = state.realtime_translation.cancellation.begin();
    let cloned_language = state.realtime_translation.language.clone();
    let cloned_volume_toggle = std::sync::Arc::clone(&state.realtime_translation.volume_toggle);
    tokio::spawn(async move {
        if let Err(e) = internal::openai_realtime(cancelled, rx, cloned_language, result_tx).await {
            eprintln!("Error occurred while running OpenAI realtime: {}", e);
        }
        cloned_volume_toggle.store(false, std::sync::atomic::Ordering::Relaxed);
    });

    let cloned_volume_toggle = std::sync::Arc::clone(&state.realtime_translation.volume_toggle);
    tokio::spawn(async move {
        if let Err(e) = internal::capture_audio(cloned_volume_toggle, tx).await {
            eprintln!("Error occurred while capturing audio: {}", e);
        }
    });

    Ok(())
}

#[tauri::command]
pub async fn stop_realtime_translation(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<(), ipc::CommandError> {
    let state = state.lock().await;
    state
        .realtime_translation
        .volume_toggle
        .store(false, std::sync::atomic::Ordering::Relaxed);
    state.realtime_translation.cancellation.cancel();
    Ok(())
}

mod internal {
    use super::*;

    use futures_util::{SinkExt, StreamExt};
    use std::io::Write;

    fn calculate_volume(buffer: &[u8]) -> f64 {
        audio::rms(&audio::utils::bytes_to_i16_samples(buffer)) / 32768.0
    }

    pub(super) async fn capture_audio(
        running: std::sync::Arc<std::sync::atomic::AtomicBool>,
        tx: tokio::sync::mpsc::Sender<Vec<u8>>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let device_name = "taurin-loopback";
        audio::prepare_loopback(
            device_name.to_string(),
            OPENAI_REALTIME_API_SAMPLING_RATE,
            OPENAI_REALTIME_API_CHANNELS,
        )?;

        let result = tokio::task::spawn_blocking(move || {
            audio::capture_device(
                device_name,
                OPENAI_REALTIME_API_SAMPLING_RATE,
                OPENAI_REALTIME_API_CHANNELS,
                move |data| {
                    dbg!(calculate_volume(data));

                    if let Err(e) = tx.blocking_send(data.to_vec()) {
                        eprintln!("Error occurred while sending audio data: {}", e);
                    }

                    if running.load(std::sync::atomic::Ordering::Relaxed) {
                        audio::CaptureControl::Continue
                    } else {
                        audio::CaptureControl::Stop
                    }
                },
            )
        })
        .await?;

        Ok(result?)
    }

    pub(super) async fn openai_realtime(
        mut cancelled: tokio::sync::watch::Receiver<u64>,
        mut rx: tokio::sync::mpsc::Receiver<Vec<u8>>,
        language: String,
        result_tx: tauri::ipc::Channel<String>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let client = bakery::Client::new(crate::commands::BAKERY_URL, 0);
        let token = tokio::select! {
            token = client.get_value(crate::commands::COOKIE_NAME) => token?,
            Ok(()) = cancelled.changed() => return Ok(()),
        };

        let url = format!(
            "{}/realtime?model={}",
            crate::commands::CORTEX_WEBSOCKET_URL,
            OPENAI_REALTIME_API_MODEL,
        )
        .parse()?;

        let request = tokio_tungstenite::tungstenite::client::ClientRequestBuilder::new(url)
            .with_header(
                "Cookie",
                format!("{}={}", crate::commands::COOKIE_NAME, token),
            );

        let connected = tokio::select! {
            connected = tokio_tungstenite::connect_async(request) => connected,
            Ok(()) = cancelled.changed() => return Ok(()),
        };

        match connected {
            Ok((ws_stream, _)) => {
                let (mut ws_tx, mut ws_rx) = ws_stream.split();

                let session_update = openai::types::realtime::Event::SessionUpdate {
                    session: openai::types::realtime::Session {
                        session_type: Some(openai::types::realtime::SessionType::Realtime),
                        audio: Some(openai::types::realtime::AudioConfiguration {
                            input: Some(openai::types::realtime::AudioInput {
                                format: Some(openai::types::realtime::AudioFormat {
                                    format_type: "audio/pcm".to_string(),
                                    rate: Some(24000),
                                }),
                                transcription: Some(
                                    openai::types::realtime::InputAudioTranscription {
                                        model: Some("gpt-4o-transcribe".to_string()),
                                        ..Default::default()
                                    },
                                ),
                                turn_detection: Some(openai::types::realtime::TurnDetection {
                                    create_response: Some(true),
                                    eagerness: None,
                                    idle_timeout_ms: None,
                                    interrupt_response: Some(true),
                                    prefix_padding_ms: Some(30),
                                    silence_duration_ms: Some(50),
                                    threshold: Some(0.1),
                                    turn_detection_type:
                                        openai::types::realtime::TurnDetectionType::ServerVad,
                                }),
                                ..Default::default()
                            }),
                            ..Default::default()
                        }),
                        instructions: Some(format!(
                            "You are a translator. Please convert user input to {}. Skip if the input is already in {}. Do not provide any output other than the translation.",
                            language, language
                        )),
                        output_modalities: vec![openai::types::realtime::Modality::Text],
                        ..Default::default()
                    },
                };
                ws_tx
                    .send(tokio_tungstenite::tungstenite::protocol::Message::text(
                        serde_json::to_string(&session_update)?,
                    ))
                    .await?;

                let cloned_result_tx = result_tx.clone();
                let receiver = tokio::spawn(async move {
                    while let Some(message) = ws_rx.next().await {
                        match message {
                            Ok(message) => {
                                if message.is_text() {
                                    let text = message.into_text().unwrap_or_default().to_string();
                                    let event = serde_json::from_str::<
                                        openai::types::realtime::Event,
                                    >(&text);
                                    match event {
                                        Ok(openai::types::realtime::Event::ResponseOutputTextDelta {
                                            delta,
                                            ..
                                        }) => {
                                            let _ = cloned_result_tx.send(delta);
                                        }
                                        Ok(openai::types::realtime::Event::ResponseOutputTextDone {
                                            ..
                                        }) => {
                                            let _ = cloned_result_tx.send("\n".to_string());
                                        }
                                        _ => {}
                                    }
                                } else if message.is_close() {
                                    break;
                                }
                            }
                            Err(e) => {
                                eprintln!("Error receiving message: {}", e);
                                break;
                            }
                        }
                    }
                });

                loop {
                    tokio::select! {
                        data = rx.recv() => {
                            match data {
                                Some(data) => {
                                    let mut writer = base64::write::EncoderStringWriter::new(
                                        &base64::engine::general_purpose::STANDARD,
                                    );
                                    writer.write_all(&data)?;

                                    let input_audio_buffer_append =
                                        openai::types::realtime::Event::InputAudioBufferAppend {
                                            audio: writer.into_inner(),
                                        };

                                    let _ = ws_tx
                                        .send(tokio_tungstenite::tungstenite::protocol::Message::text(
                                            serde_json::to_string(&input_audio_buffer_append)?,
                                        ))
                                        .await;
                                }
                                None => break,
                            }
                        }
                        Ok(()) = cancelled.changed() => break,
                    }
                }

                receiver.abort();
            }
            Err(e) => {
                return Err(format!("Failed to connect to WebSocket: {}", e).into());
            }
        }

        Ok(())
    }
}
