use std::io::Write;

pub mod realtime_translation;
pub mod settings;
pub mod tmux_monitor;
pub mod voice_input;

pub const COOKIE_NAME: &str = "_oauth2_proxy";
pub const BAKERY_URL: &str = "https://bakery.kaidotio.dev/callback";
pub const CORTEX_API_URL: &str = "https://cortex-api.kaidotio.dev/v1";
pub const CORTEX_WEBSOCKET_URL: &str = "wss://cortex-api.kaidotio.dev/v1";

#[tauri::command]
pub async fn bake(
    _app_handle: tauri::AppHandle,
    state: tauri::State<'_, tokio::sync::Mutex<crate::AppState>>,
) -> Result<String, ipc::CommandError> {
    let mut cancelled = state.lock().await.bake.cancellation.begin();

    let client = bakery::Client::new(BAKERY_URL, 0);
    tokio::select! {
        value = client.get_value(COOKIE_NAME) => Ok(value?),
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
        let mut contents = Vec::new();
        for monitor in xcap::Monitor::all()? {
            // spawn_blocking keeps running after the caller drops its handle, so the loop has to notice the cancellation itself
            if cancelled.has_changed()? {
                break;
            }
            let mut buffer = std::io::Cursor::new(Vec::new());
            let captured = monitor.capture_image()?;
            let resized = downscale_dynamic_image(captured.into(), 1920, 1080)?;
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

    fn downscale_dynamic_image(
        image: image::DynamicImage,
        max_width: u32,
        max_height: u32,
    ) -> Result<image::DynamicImage, image::ImageError> {
        use image::GenericImageView;

        let (width, height) = image.dimensions();

        let (new_width, new_height) = if width > max_width || height > max_height {
            let width_ratio = max_width as f32 / width as f32;
            let height_ratio = max_height as f32 / height as f32;
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

        Ok(resized)
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
