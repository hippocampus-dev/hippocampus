#[derive(Clone, Debug, serde::Deserialize)]
pub struct AudioSpeechRequest {
    pub model: String,
    pub input: String,
    /// Accepted for OpenAI compatibility; the pipeline's speaker asset defines
    /// its single voice, so the value is ignored. `/v1/chat/completions` names
    /// a pipeline here instead, which is the closer reading of the field.
    #[serde(default)]
    pub voice: Option<String>,
    /// "wav" (default) or "pcm" (raw signed 16-bit little-endian, 24 kHz mono).
    #[serde(default)]
    pub response_format: Option<String>,
    pub temperature: Option<f32>,
    pub seed: Option<u32>,
}

/// OpenAI-compatible text-to-audio endpoint backed by a Thinker-Talker audio
/// pipeline (`[audio_pipelines]` in models.toml).
pub async fn audio_speech(
    axum::extract::State(state): axum::extract::State<crate::AppState>,
    axum::extract::Json(request): axum::extract::Json<AudioSpeechRequest>,
) -> axum::response::Response<axum::body::Body> {
    let _ = &request.voice;

    if request.input.is_empty() {
        return crate::handler::chat_completions::openai_error_response(
            axum::http::StatusCode::BAD_REQUEST,
            "'input' must not be empty",
            "invalid_request_error",
        );
    }
    // Refused here rather than left to `generate`, which is the central check
    // both endpoints share but only runs after the pipeline is loaded and its
    // mutex taken. Loading is up to three GGUFs off disk and can evict a warm
    // pipeline to make room, so an over-long request that is going to be
    // refused anyway should not cost that first -- nor make a concurrent
    // legitimate request wait behind it.
    let characters = request.input.chars().count();
    if characters > crate::audio::pipeline::MAX_INPUT_CHARACTERS {
        return crate::handler::chat_completions::openai_error_response(
            axum::http::StatusCode::BAD_REQUEST,
            &format!(
                "Input is too long: {} characters exceeds the limit of {}",
                characters,
                crate::audio::pipeline::MAX_INPUT_CHARACTERS
            ),
            "invalid_request_error",
        );
    }

    let response_format = request.response_format.as_deref().unwrap_or("wav");
    let (content_type, as_wav) = match response_format {
        "wav" => ("audio/wav", true),
        "pcm" => ("application/octet-stream", false),
        _ => {
            return crate::handler::chat_completions::openai_error_response(
                axum::http::StatusCode::BAD_REQUEST,
                &format!("Unsupported response_format '{response_format}'; use \"wav\" or \"pcm\""),
                "invalid_request_error",
            );
        }
    };

    let pipeline = match state
        .llama_backend
        .get_or_load_audio_pipeline(&request.model)
        .await
    {
        Ok(pipeline) => pipeline,
        // Every pipeline is busy: the voice exists and the same request may
        // succeed once one finishes, so saying it does not exist would be a
        // lie the client cannot act on.
        Err(crate::model_manager::LoadError::AtCapacity(e)) => {
            return crate::handler::chat_completions::openai_error_response(
                axum::http::StatusCode::SERVICE_UNAVAILABLE,
                &format!("{e}"),
                "server_error",
            );
        }
        Err(crate::model_manager::LoadError::NotFound(e)) => {
            return crate::handler::chat_completions::openai_error_response(
                axum::http::StatusCode::NOT_FOUND,
                &format!("{e}"),
                "model_not_found",
            );
        }
        Err(crate::model_manager::LoadError::Failed(e)) => {
            return crate::handler::chat_completions::openai_error_response(
                axum::http::StatusCode::NOT_FOUND,
                &format!("Failed to load audio pipeline '{}': {}", request.model, e),
                "model_not_found",
            );
        }
    };

    // Generation is seconds of CPU-bound work (LLM pass, projection matmuls,
    // iSTFT); run it off the async workers. The async mutex is acquired
    // before entering the blocking pool so queued requests wait without
    // pinning the pool's few threads.
    let mut guard = pipeline.lock_owned().await;
    let input = request.input;
    let temperature = request.temperature;
    let seed = request.seed;
    let generated =
        tokio::task::spawn_blocking(move || guard.generate(&input, temperature, seed)).await;

    let output = match generated {
        Ok(Ok(output)) => output,
        Ok(Err(crate::audio::pipeline::GenerateError::InvalidInput(message))) => {
            return crate::handler::chat_completions::openai_error_response(
                axum::http::StatusCode::BAD_REQUEST,
                &message,
                "invalid_request_error",
            );
        }
        Ok(Err(crate::audio::pipeline::GenerateError::Internal(e))) => {
            return crate::handler::chat_completions::openai_error_response(
                axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                &format!("Audio generation failed: {e}"),
                "generation_error",
            );
        }
        Err(e) => {
            return crate::handler::chat_completions::openai_error_response(
                axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                &format!("Audio generation task failed: {e}"),
                "internal_error",
            );
        }
    };

    let body = if as_wav {
        crate::audio::pipeline::encode_wav(&output.samples, output.sample_rate)
    } else {
        crate::audio::pipeline::encode_pcm16(&output.samples)
    };

    axum::response::Response::builder()
        .status(axum::http::StatusCode::OK)
        .header("Content-Type", content_type)
        // Truncated speech stops mid-utterance and the samples do not show it,
        // so a caller reading only the body cannot tell it from a short one.
        // The body is audio and has nowhere to say so; this is the only channel
        // this endpoint has.
        .header("Audio-Complete", output.complete.to_string())
        .body(axum::body::Body::from(body))
        .unwrap()
}
