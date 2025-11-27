const OPENAI_REALTIME_API_SAMPLING_RATE: u32 = 24000;
const OPENAI_REALTIME_API_CHANNELS: u8 = 1;

#[tauri::command]
pub async fn start_realtime_translation(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
    result_tx: tauri::ipc::Channel<String>,
) -> Result<(), String> {
    let state = state.lock().await;
    if state
        .realtime_translation
        .volume_toggle
        .load(std::sync::atomic::Ordering::Relaxed)
    {
        return Err("already running".to_string());
    }
    state
        .realtime_translation
        .volume_toggle
        .store(true, std::sync::atomic::Ordering::Relaxed);

    let (tx, rx) = tokio::sync::mpsc::channel::<Vec<u8>>(10);

    let cloned_volume_toggle = std::sync::Arc::clone(&state.realtime_translation.volume_toggle);
    let cloned_language = state.realtime_translation.language.clone();
    tokio::spawn(async move {
        if let Err(e) =
            internal::openai_realtime(cloned_volume_toggle, rx, cloned_language, result_tx).await
        {
            eprintln!("Error occurred while running OpenAI realtime: {}", e);
        }
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
) -> Result<(), String> {
    let state = state.lock().await;
    state
        .realtime_translation
        .volume_toggle
        .store(false, std::sync::atomic::Ordering::Relaxed);
    Ok(())
}

mod internal {
    use super::*;

    use futures_util::{SinkExt, StreamExt};
    use std::io::Write;

    fn calculate_volume(buffer: &[u8]) -> f32 {
        let mut sum = 0.0;

        for i in (0..buffer.len()).step_by(2) {
            if i + 1 < buffer.len() {
                let sample = i16::from_le_bytes([buffer[i], buffer[i + 1]]);
                let normalized = f32::from(sample) / 32768.0;
                sum += normalized * normalized;
            }
        }

        (sum / (buffer.len() as f32 / 2.0)).sqrt()
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
        running: std::sync::Arc<std::sync::atomic::AtomicBool>,
        mut rx: tokio::sync::mpsc::Receiver<Vec<u8>>,
        language: String,
        result_tx: tauri::ipc::Channel<String>,
    ) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
        let client = bakery::Client::new(crate::commands::BAKERY_URL, 0);
        let token = client.get_value(crate::commands::COOKIE_NAME).await?;

        let url = format!(
            "{}/realtime?model=gpt-4o-realtime-preview-2024-12-17",
            crate::commands::CORTEX_WEBSOCKET_URL,
        )
        .parse()?;

        let request = tokio_tungstenite::tungstenite::client::ClientRequestBuilder::new(url)
            .with_header(
                "Cookie",
                format!("{}={}", crate::commands::COOKIE_NAME, token),
            )
            .with_header("OpenAI-Beta", "realtime=v1");

        match tokio_tungstenite::connect_async(request).await {
            Ok((ws_stream, _)) => {
                let (mut ws_tx, mut ws_rx) = ws_stream.split();

                let session_update = openai::types::realtime::Event::SessionUpdate {
                    session: openai::types::realtime::Session {
                        modalities: vec![openai::types::realtime::Modality::Text],
                        instructions: Some(
                            format!("You are a translator. Please convert user input to {}. Skip if the input is already in {}. Do not provide any output other than the translation.", language, language),
                        ),
                        input_audio_format: Some("pcm16".to_string()),
                        turn_detection: Some(openai::types::realtime::TurnDetection {
                            create_response: true,
                            interrupt_response: true,
                            prefix_padding_ms: 30,
                            silence_duration_ms: 50,
                            threshold: 0.1,
                            r#type: "server_vad".to_string(),
                        }),
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
                                        Ok(openai::types::realtime::Event::ResponseTextDelta {
                                            delta,
                                            ..
                                        }) => {
                                            let _ = cloned_result_tx.send(delta);
                                        }
                                        Ok(openai::types::realtime::Event::ResponseTextDone {
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

                while running.load(std::sync::atomic::Ordering::Relaxed) {
                    if let Some(data) = rx.recv().await {
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
                    } else {
                        break;
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
