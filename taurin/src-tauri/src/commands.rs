use std::io::Write;

pub mod auto_update;
pub mod create_github_repositories;
pub mod overlay;
pub mod realtime_translation;
pub mod settings;
pub mod spotify;
pub mod tmux_monitor;
#[cfg(feature = "cuda")]
pub mod voice_input;

pub const COOKIE_NAME: &str = "_oauth2_proxy";
pub const BAKERY_URL: &str = "https://bakery.kaidotio.dev/callback";
pub const TAURI_RELEASES_URL: &str = "https://tauri-releases-public.kaidotio.dev";
pub const CORTEX_API_URL: &str = "https://cortex-api.kaidotio.dev/v1";
pub const CORTEX_WEBSOCKET_URL: &str = "wss://cortex-api.kaidotio.dev/v1";
pub const GITHUB_CREDENTIAL_PROXY_URL: &str = "https://github-credential-proxy.kaidotio.dev";
pub const SPOTIFY_OAUTH_BRIDGE_URL: &str =
    "https://hippocampus-dev-spotify-oauth-bridge.kaidotio.dev";
pub const SPOTIFY_API_URL: &str = "https://api.spotify.com/v1";

const PING_EXTENSION_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(5);

#[tauri::command]
pub async fn bake(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<String, ipc::CommandError> {
    let mut cancelled = state.lock().await.bake.cancellation.begin();

    let client = bakery::Client::new(BAKERY_URL, 0);
    tokio::select! {
        value = client.get_value(COOKIE_NAME) => Ok(value.map_err(|e| e.to_string())?),
        Ok(()) = cancelled.changed() => Err(ipc::CommandError::Cancelled),
    }
}

#[tauri::command]
pub async fn cancel_bake(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<(), ipc::CommandError> {
    state.lock().await.bake.cancellation.cancel();
    Ok(())
}

#[tauri::command]
pub async fn explain_monitors(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<String, ipc::CommandError> {
    let mut cancelled = state.lock().await.explain_monitors.cancellation.begin();

    tokio::select! {
        explanation = internal::explain_monitors(cancelled.clone()) => Ok(explanation?),
        Ok(()) = cancelled.changed() => Err(ipc::CommandError::Cancelled),
    }
}

#[tauri::command]
pub async fn cancel_explain_monitors(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<(), ipc::CommandError> {
    state.lock().await.explain_monitors.cancellation.cancel();
    Ok(())
}

#[tauri::command]
#[ipc::estringify]
pub async fn save_window_state(
    app_handle: tauri::AppHandle,
) -> Result<(), Box<dyn std::error::Error + Send + Sync + 'static>> {
    tauri_plugin_window_state::AppHandleExt::save_window_state(
        &app_handle,
        tauri_plugin_window_state::StateFlags::default(),
    )?;

    Ok(())
}

#[tauri::command]
#[ipc::estringify]
pub async fn ping_extension(
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<String, Box<dyn std::error::Error + Send + Sync + 'static>> {
    let bridge = state
        .lock()
        .await
        .native_messaging
        .clone()
        .ok_or("native messaging bridge is not running")?;
    let response = bridge
        .send(
            serde_json::json!({ "type": "ping" }),
            PING_EXTENSION_TIMEOUT,
        )
        .await?;
    Ok(serde_json::to_string_pretty(&response)?)
}

mod internal {
    use super::*;

    pub(super) async fn explain_monitors(
        cancelled: tokio::sync::watch::Receiver<u64>,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let mut messages = vec![openai::types::chat::RequestMessage {
            role: openai::types::chat::Role::System,
            content: openai::types::chat::Content::String(
                "この画像に写っている問題を解決する方法を教えてください。些細な問題は無視して、根本的な問題にのみ焦点を当ててください。".to_string(),
            ),
        }];

        let contents = tokio::task::spawn_blocking(move || capture_monitors(cancelled)).await??;

        messages.push(openai::types::chat::RequestMessage {
            role: openai::types::chat::Role::User,
            content: openai::types::chat::Content::Typed(contents),
        });

        let client = bakery::Client::new(BAKERY_URL, 0);
        let token = client.get_value(COOKIE_NAME).await?;

        let mut builder = openai::Client::builder();
        builder.set_openai_api_base(CORTEX_API_URL.parse()?);
        builder.set_connect_timeout(std::time::Duration::from_millis(100));
        builder.set_retry_strategy(Box::new(
            openai::strategy::JitteredExponentialBackoff::new(std::time::Duration::from_millis(
                100,
            ))
            .take(3),
        ));
        let client = builder.build(format!("{}={}", COOKIE_NAME, token))?;
        let body = openai::types::chat::CompletionRequestBody {
            model: "gpt-5.4".to_string(),
            messages,
            n: 1,
            ..Default::default()
        };
        let result = client
            .post("/chat/completions", serde_json::to_vec(&body)?)
            .await?;

        let response: openai::types::chat::CompletionResponseBody =
            serde_json::from_slice(&result)?;

        Ok(response.choices[0].message.content.to_string())
    }

    fn capture_monitors(
        cancelled: tokio::sync::watch::Receiver<u64>,
    ) -> Result<
        Vec<openai::types::chat::TypedContent>,
        Box<dyn std::error::Error + Send + Sync + 'static>,
    > {
        use image::GenericImageView;

        let mut contents = Vec::new();
        for monitor in xcap::Monitor::all()? {
            // spawn_blocking keeps running after the caller drops its handle, so the loop has to notice the cancellation itself
            if cancelled.has_changed()? {
                break;
            }
            let mut buffer = std::io::Cursor::new(Vec::new());
            let image: image::DynamicImage = monitor.capture_image()?.into();
            let (width, height) = image.dimensions();

            let (new_width, new_height) = if width > 1920 || height > 1080 {
                let width_ratio = 1920.0 / width as f32;
                let height_ratio = 1080.0 / height as f32;
                let scale = width_ratio.min(height_ratio);
                (
                    (width as f32 * scale).round() as u32,
                    (height as f32 * scale).round() as u32,
                )
            } else {
                (width, height)
            };

            let resized = if width != new_width || height != new_height {
                image.resize(new_width, new_height, image::imageops::FilterType::Nearest)
            } else {
                image
            };
            resized.write_to(&mut buffer, image::ImageFormat::Png)?;

            let mut writer =
                base64::write::EncoderStringWriter::new(&base64::engine::general_purpose::STANDARD);
            writer.write_all(buffer.get_ref())?;
            contents.push(openai::types::chat::TypedContent::ImageUrl {
                image_url: openai::types::chat::MessageContentTypedImageUrl {
                    url: format!("data:image/png;base64,{}", writer.into_inner()),
                },
            })
        }
        Ok(contents)
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[tokio::test]
        #[ignore]
        async fn test_explain_monitors() {
            let result = explain_monitors(tokio::sync::watch::channel(0).1).await;
            assert!(result.is_ok());
        }
    }
}
