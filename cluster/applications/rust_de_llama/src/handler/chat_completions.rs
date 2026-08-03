use rand::Rng;

#[derive(Clone, Debug, serde::Deserialize)]
pub struct GenerateRequest {
    pub prompt: String,
    pub max_tokens: Option<u32>,
    pub temperature: Option<f32>,
    pub top_k: Option<i32>,
    pub top_p: Option<f32>,
    pub frequency_penalty: Option<f32>,
    pub presence_penalty: Option<f32>,
    pub seed: Option<u32>,
}

#[derive(Clone, Debug, serde::Deserialize)]
pub struct ChatCompletionRequest {
    pub model: String,
    pub messages: Vec<Message>,
    pub temperature: Option<f32>,
    pub top_k: Option<i32>,
    pub top_p: Option<f32>,
    pub max_tokens: Option<u32>,
    pub stream: Option<bool>,
    pub frequency_penalty: Option<f32>,
    pub presence_penalty: Option<f32>,
    pub seed: Option<u32>,
    pub stop: Option<StopParameter>,
    #[serde(default)]
    pub stream_options: Option<StreamOptions>,
    /// `["text", "audio"]` speaks the generated reply. "text" alone is the
    /// default; the reply is always returned as text either way.
    #[serde(default)]
    pub modalities: Option<Vec<String>>,
    #[serde(default)]
    pub audio: Option<AudioParameter>,
}

/// Audio options for `modalities: ["text", "audio"]`.
#[derive(Clone, Debug, serde::Deserialize)]
pub struct AudioParameter {
    /// `[audio_pipelines]` name in models.toml. OpenAI names a voice here, and
    /// a pipeline is one: its speaker prompt is what the talker imitates.
    pub voice: String,
    /// "wav" (default) or "pcm" (raw signed 16-bit little-endian, 24 kHz mono).
    #[serde(default)]
    pub format: Option<String>,
}

/// Streaming-only options. Only `include_usage` is honoured.
#[derive(Clone, Debug, serde::Deserialize)]
pub struct StreamOptions {
    #[serde(default)]
    pub include_usage: Option<bool>,
}

#[derive(Clone, Debug, serde::Deserialize)]
#[serde(untagged)]
pub enum StopParameter {
    Single(String),
    Multiple(Vec<String>),
}

impl StopParameter {
    fn into_sequences(self) -> Vec<String> {
        match self {
            StopParameter::Single(stop) => vec![stop],
            StopParameter::Multiple(stops) => stops,
        }
    }
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Message {
    pub role: String,
    pub content: String,
    /// The spoken reply. Response-only: `Message` is also the request's
    /// `messages` element, and OpenAI has clients replay an assistant audio
    /// turn as `{"role": "assistant", "audio": {"id": "..."}}`, which would not
    /// deserialize into `AudioContent`. Absent when synthesis was not asked
    /// for, and when it was asked for but the reply could not be spoken.
    #[serde(skip_deserializing, skip_serializing_if = "Option::is_none")]
    pub audio: Option<AudioContent>,
}

/// The generated reply rendered to speech.
#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct AudioContent {
    pub id: String,
    /// Unix seconds after which the audio is no longer retrievable. Required by
    /// OpenAI's schema; nothing is stored here, so it is the response time.
    pub expires_at: u64,
    /// Base64 of the requested `format`.
    pub data: String,
    /// The text that was spoken. It is `content` verbatim; the talker's own
    /// normalization is not reflected here.
    pub transcript: String,
}

#[derive(Clone, Debug, serde::Serialize)]
pub struct ChatCompletionResponse {
    pub id: String,
    pub object: String,
    pub created: u64,
    pub model: String,
    pub choices: Vec<Choice>,
    pub usage: Usage,
}

#[derive(Clone, Debug, serde::Serialize)]
pub struct ChatCompletionChunk {
    pub id: String,
    pub object: String,
    pub created: u64,
    pub model: String,
    pub choices: Vec<DeltaChoice>,
    /// Token counts for the whole request, carried on the terminating chunk so a
    /// streaming client can account tokens without a second request. Absent on
    /// every earlier chunk, where the totals are not yet known.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<Usage>,
}

#[derive(Clone, Debug, serde::Serialize)]
pub struct Choice {
    pub index: usize,
    pub message: Message,
    pub finish_reason: String,
}

#[derive(Clone, Debug, serde::Serialize)]
pub struct DeltaChoice {
    pub index: usize,
    pub delta: DeltaMessage,
    pub finish_reason: Option<String>,
}

#[derive(Clone, Debug, serde::Serialize)]
pub struct DeltaMessage {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub role: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<String>,
}

#[derive(Clone, Debug, serde::Serialize)]
pub struct Usage {
    pub prompt_tokens: u32,
    pub completion_tokens: u32,
    pub total_tokens: u32,
}

pub async fn chat_completions(
    axum::extract::State(state): axum::extract::State<crate::AppState>,
    axum::extract::Json(request): axum::extract::Json<ChatCompletionRequest>,
) -> axum::response::Response<axum::body::Body> {
    let mut wants_audio = false;
    for modality in request.modalities.iter().flatten() {
        match modality.as_str() {
            "audio" => wants_audio = true,
            "text" => {}
            modality => {
                return openai_error_response(
                    axum::http::StatusCode::BAD_REQUEST,
                    &format!("Unsupported modality '{modality}'; use \"text\" or \"audio\""),
                    "invalid_request_error",
                );
            }
        }
    }
    let audio_parameter = match (wants_audio, &request.audio) {
        (true, None) => {
            return openai_error_response(
                axum::http::StatusCode::BAD_REQUEST,
                "'audio' is required when 'modalities' includes \"audio\"",
                "invalid_request_error",
            );
        }
        (true, Some(audio)) => {
            // The reply has to be complete before it can be spoken, so the two
            // cannot be streamed together yet.
            if request.stream.unwrap_or(false) {
                return openai_error_response(
                    axum::http::StatusCode::BAD_REQUEST,
                    "'audio' is not supported with 'stream'; synthesis needs the whole reply",
                    "invalid_request_error",
                );
            }
            match audio.format.as_deref().unwrap_or("wav") {
                "wav" | "pcm" => {}
                format => {
                    return openai_error_response(
                        axum::http::StatusCode::BAD_REQUEST,
                        &format!("Unsupported audio format '{format}'; use \"wav\" or \"pcm\""),
                        "invalid_request_error",
                    );
                }
            }
            audio.clone()
        }
        (false, _) => AudioParameter {
            voice: String::new(),
            format: None,
        },
    };

    let model_name = &request.model;
    let processor = match state.llama_backend.get_or_load_model(model_name).await {
        Ok(p) => p,
        // Every resident model is busy: the name is fine and the same request
        // may well succeed once one finishes, so this is a 503 rather than a
        // 404 claiming the model does not exist.
        Err(crate::model_manager::LoadError::AtCapacity(e)) => {
            return openai_error_response(
                axum::http::StatusCode::SERVICE_UNAVAILABLE,
                &format!("{e}"),
                "server_error",
            );
        }
        Err(e) => {
            let error_response = serde_json::json!({
                "error": {
                    "message": format!("Failed to load model '{}': {}", model_name, e),
                    "type": "model_not_found",
                    "param": "model",
                    "code": null
                }
            });

            return axum::response::Response::builder()
                .status(axum::http::StatusCode::NOT_FOUND)
                .header("Content-Type", "application/json")
                .body(axum::body::Body::from(
                    serde_json::to_string(&error_response).unwrap(),
                ))
                .unwrap();
        }
    };

    // Resolved before generating, so a voice that does not exist costs the
    // client an error rather than a whole completion first -- but after the
    // chat model, so a thinker shared with it is already resident and is not
    // loaded a second time.
    let audio_pipeline = if wants_audio {
        match state
            .llama_backend
            .get_or_load_audio_pipeline(&audio_parameter.voice)
            .await
        {
            Ok(pipeline) => Some(pipeline),
            Err(crate::model_manager::LoadError::AtCapacity(e)) => {
                return openai_error_response(
                    axum::http::StatusCode::SERVICE_UNAVAILABLE,
                    &format!("{e}"),
                    "server_error",
                );
            }
            Err(crate::model_manager::LoadError::NotFound(e)) => {
                return openai_error_response(
                    axum::http::StatusCode::NOT_FOUND,
                    &format!("{e}"),
                    "model_not_found",
                );
            }
            Err(crate::model_manager::LoadError::Failed(e)) => {
                return openai_error_response(
                    axum::http::StatusCode::NOT_FOUND,
                    &format!(
                        "Failed to load audio pipeline '{}': {}",
                        audio_parameter.voice, e
                    ),
                    "model_not_found",
                );
            }
        }
    } else {
        None
    };

    let model_config = state.llama_backend.get_model_config(model_name).await;
    let prompt = {
        let mut formatted = String::new();
        let fmt = &model_config.prompt_format;

        let has_format = fmt.user_prefix.is_some()
            || fmt.assistant_prefix.is_some()
            || fmt.system_prefix.is_some();

        if has_format {
            for message in &request.messages {
                match message.role.as_str() {
                    "system" => {
                        if let Some(prefix) = &fmt.system_prefix {
                            formatted.push_str(prefix);
                        }
                        formatted.push_str(&message.content);
                        if let Some(suffix) = &fmt.system_suffix {
                            formatted.push_str(suffix);
                        }
                    }
                    "user" => {
                        if let Some(prefix) = &fmt.user_prefix {
                            formatted.push_str(prefix);
                        }
                        formatted.push_str(&message.content);
                        if let Some(suffix) = &fmt.user_suffix {
                            formatted.push_str(suffix);
                        }
                    }
                    "assistant" => {
                        if let Some(prefix) = &fmt.assistant_prefix {
                            formatted.push_str(prefix);
                        }
                        formatted.push_str(&message.content);
                        if let Some(suffix) = &fmt.assistant_suffix {
                            formatted.push_str(suffix);
                        }
                    }
                    _ => {
                        formatted.push_str(&format!("{}:\n{}\n", message.role, message.content));
                    }
                }
            }

            if let Some(gen_prompt) = &fmt.add_generation_prompt {
                formatted.push_str(gen_prompt);
            }
        } else {
            for message in &request.messages {
                formatted.push_str(&format!("{}:\n{}\n", message.role, message.content));
            }
        }

        tracing::debug!(
            "Formatted prompt for model '{}': {:?}",
            model_name,
            formatted
        );

        formatted
    };

    // Reject prompts whose tokenized length exceeds the admission bound before
    // submitting the task, converting silently-truncated (wrong) answers into a
    // clean 400.
    let prompt_tokens = match processor.tokenize_prompt_for_admission(&prompt) {
        Ok(tokens) => tokens,
        Err(e) => {
            return openai_error_response(
                axum::http::StatusCode::BAD_REQUEST,
                &format!("Failed to tokenize prompt: {e}"),
                "invalid_request_error",
            );
        }
    };
    let prompt_token_count = prompt_tokens.len();
    let max_prompt_tokens = processor.max_prompt_tokens(model_config.max_prompt_tokens);
    if prompt_token_count as i32 > max_prompt_tokens {
        return openai_error_response(
            axum::http::StatusCode::BAD_REQUEST,
            &format!(
                "Prompt is too long: {prompt_token_count} tokens exceeds the limit of {max_prompt_tokens}"
            ),
            "invalid_request_error",
        );
    }

    let stream = request.stream.unwrap_or(false);
    let id = format!(
        "chatcmpl-{}",
        rand::thread_rng()
            .sample_iter(rand::distributions::Alphanumeric)
            .take(29)
            .map(char::from)
            .collect::<String>()
    );
    let created = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs();

    // Size the channel to the maximum event count a healthy generation can
    // produce (tokens + Complete, with margin) so the scheduling loop's
    // try_send can never fill for a well-behaved client.
    let effective_max_tokens = request
        .max_tokens
        .map(|t| t as usize)
        .unwrap_or(crate::parallel::DEFAULT_MAX_TOKENS);
    let channel_capacity = effective_max_tokens
        .min(processor.n_ctx_seq().max(0) as usize)
        .saturating_add(2)
        .max(1);
    let (tx, mut rx) = tokio::sync::mpsc::channel::<
        Result<crate::parallel::TaskResponse, error::Error>,
    >(channel_capacity);

    let idle_timeout = model_config
        .request_idle_timeout_seconds
        .map(std::time::Duration::from_secs);

    // Merge request-supplied stop sequences with the model-configured ones.
    let mut stop_sequences = model_config.stop_sequences.clone().unwrap_or_default();
    if let Some(stop) = request.stop.clone() {
        stop_sequences.extend(stop.into_sequences());
    }
    let stop_sequences = if stop_sequences.is_empty() {
        None
    } else {
        Some(stop_sequences)
    };

    let task = crate::parallel::Task {
        id: id.clone(),
        request: GenerateRequest {
            prompt,
            max_tokens: request.max_tokens,
            temperature: request.temperature,
            top_k: request.top_k,
            top_p: request.top_p,
            frequency_penalty: request.frequency_penalty,
            presence_penalty: request.presence_penalty,
            seed: request.seed,
        },
        prompt_tokens,
        response_tx: tx,
        stop: stop_sequences,
    };

    if let Err(e) = processor.submit_task(task) {
        return openai_error_response(
            axum::http::StatusCode::INTERNAL_SERVER_ERROR,
            &format!("Failed to submit task: {e}"),
            "internal_error",
        );
    }

    if stream {
        let id_for_stream = id.clone();
        let model = request.model.clone();
        let include_usage = request
            .stream_options
            .as_ref()
            .and_then(|options| options.include_usage)
            .unwrap_or(false);
        let stream = async_stream::stream! {
            let mut total_prompt_tokens = 0u32;
            let mut total_completion_tokens = 0u32;

            let initial_chunk = ChatCompletionChunk {
                id: id_for_stream.clone(),
                object: "chat.completion.chunk".to_string(),
                created,
                model: model.clone(),
                choices: vec![DeltaChoice {
                    index: 0,
                    delta: DeltaMessage {
                        role: Some("assistant".to_string()),
                        content: None,
                    },
                    finish_reason: None,
                }],
                usage: None,
            };

            let data = format!("data: {}\n\n", serde_json::to_string(&initial_chunk).unwrap());
            yield Ok::<_, std::convert::Infallible>(data);

            loop {
                let message = match idle_timeout {
                    Some(duration) => match tokio::time::timeout(duration, rx.recv()).await {
                        Ok(message) => message,
                        Err(_) => {
                            let error_chunk = serde_json::json!({
                                "error": {
                                    "message": "Request timed out waiting for the next token",
                                    "type": "timeout",
                                    "param": null,
                                    "code": null
                                }
                            });
                            let data = format!("data: {}\n\n", serde_json::to_string(&error_chunk).unwrap());
                            yield Ok(data);
                            break;
                        }
                    },
                    None => rx.recv().await,
                };

                // A terminal event (Complete / Err / timeout) always breaks the
                // loop, so a None here means the channel closed without Complete:
                // the scheduler dropped the task. Emit a terminal error chunk (and
                // no [DONE]) instead of ending the stream silently.
                let Some(response) = message else {
                    let error_chunk = serde_json::json!({
                        "error": {
                            "message": "Generation ended unexpectedly",
                            "type": "internal_error",
                            "param": null,
                            "code": null
                        }
                    });
                    let data = format!("data: {}\n\n", serde_json::to_string(&error_chunk).unwrap());
                    yield Ok(data);
                    break;
                };

                match response {
                    Ok(crate::parallel::TaskResponse::Token(token)) => {
                        let chunk = ChatCompletionChunk {
                            id: id_for_stream.clone(),
                            object: "chat.completion.chunk".to_string(),
                            created,
                            model: model.clone(),
                            choices: vec![DeltaChoice {
                                index: 0,
                                delta: DeltaMessage {
                                    role: None,
                                    content: Some(token),
                                },
                                finish_reason: None,
                            }],
                            usage: None,
                        };

                        let data = format!("data: {}\n\n", serde_json::to_string(&chunk).unwrap());
                        yield Ok(data);
                    }
                    Ok(crate::parallel::TaskResponse::Complete { prompt_tokens: pt, completion_tokens: ct, finish_reason }) => {
                        total_prompt_tokens = pt;
                        total_completion_tokens = ct;

                        let final_chunk = ChatCompletionChunk {
                            id: id_for_stream.clone(),
                            object: "chat.completion.chunk".to_string(),
                            created,
                            model: model.clone(),
                            choices: vec![DeltaChoice {
                                index: 0,
                                delta: DeltaMessage {
                                    role: None,
                                    content: None,
                                },
                                finish_reason: Some(finish_reason.to_string()),
                            }],
                            usage: None,
                        };

                        let data = format!("data: {}\n\n", serde_json::to_string(&final_chunk).unwrap());
                        yield Ok(data);

                        // OpenAI carries streamed token counts on one extra
                        // chunk with an empty `choices`, sent only when the
                        // request opted in, so a client keying on `usage` never
                        // has to look for a delta beside it.
                        if include_usage {
                            let usage_chunk = ChatCompletionChunk {
                                id: id_for_stream.clone(),
                                object: "chat.completion.chunk".to_string(),
                                created,
                                model: model.clone(),
                                choices: Vec::new(),
                                usage: Some(Usage {
                                    prompt_tokens: pt,
                                    completion_tokens: ct,
                                    total_tokens: pt + ct,
                                }),
                            };

                            let data = format!("data: {}\n\n", serde_json::to_string(&usage_chunk).unwrap());
                            yield Ok(data);
                        }

                        yield Ok("data: [DONE]\n\n".to_string());

                        break;
                    }
                    Err(e) => {
                        let error_chunk = serde_json::json!({
                            "error": {
                                "message": format!("Generation error: {e}"),
                                "type": "generation_error",
                                "param": null,
                                "code": null
                            }
                        });

                        let data = format!("data: {}\n\n", serde_json::to_string(&error_chunk).unwrap());
                        yield Ok(data);

                        break;
                    }
                }
            }

            state.processed_tokens_counter.add(
                &opentelemetry::Context::current(),
                (total_prompt_tokens + total_completion_tokens) as u64,
                &[]
            );
        };

        let stream_body = axum::body::Body::from_stream(stream);

        axum::response::Response::builder()
            .status(axum::http::StatusCode::OK)
            .header("Content-Type", "text/event-stream")
            .header("Cache-Control", "no-cache")
            .header("Connection", "keep-alive")
            .body(stream_body)
            .unwrap()
    } else {
        let mut content_buffer = String::new();
        let mut prompt_tokens = 0u32;
        let mut completion_tokens = 0u32;
        let mut finish_reason = "stop";
        let mut completed = false;

        loop {
            let message = match idle_timeout {
                Some(duration) => match tokio::time::timeout(duration, rx.recv()).await {
                    Ok(message) => message,
                    Err(_) => {
                        return openai_error_response(
                            axum::http::StatusCode::GATEWAY_TIMEOUT,
                            "Request timed out waiting for the next token",
                            "timeout",
                        );
                    }
                },
                None => rx.recv().await,
            };

            let Some(response) = message else {
                break;
            };

            match response {
                Ok(crate::parallel::TaskResponse::Token(token)) => {
                    content_buffer.push_str(&token);
                }
                Ok(crate::parallel::TaskResponse::Complete {
                    prompt_tokens: pt,
                    completion_tokens: ct,
                    finish_reason: reason,
                }) => {
                    prompt_tokens = pt;
                    completion_tokens = ct;
                    finish_reason = reason;
                    completed = true;
                    break;
                }
                Err(e) => {
                    return openai_error_response(
                        axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                        &format!("Generation error: {e}"),
                        "generation_error",
                    );
                }
            }
        }

        // The channel closed without a Complete — the scheduler dropped the task.
        // Report an honest 500 rather than fabricating an empty 200.
        if !completed {
            return openai_error_response(
                axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                "Generation ended unexpectedly",
                "internal_error",
            );
        }

        state.processed_tokens_counter.add(
            &opentelemetry::Context::current(),
            (prompt_tokens + completion_tokens) as u64,
            &[],
        );

        // The reply is spoken after it is complete: a server-side join of this
        // endpoint and /v1/audio/speech. A reply the talker cannot read -- no
        // speakable characters survive normalization, or it outruns the talker's
        // context -- comes back as text with no audio rather than not at all:
        // the completion is already generated and metered, and failing it would
        // only have the client pay to retry into the same deterministic wall.
        let audio = match audio_pipeline {
            Some(pipeline) => {
                match speak_reply(
                    pipeline,
                    &audio_parameter,
                    &content_buffer,
                    request.seed,
                    &id,
                    created,
                )
                .await
                {
                    Ok(audio) => Some(audio),
                    Err(e) => {
                        tracing::error!("Failed to speak the generated reply: {}", e);
                        None
                    }
                }
            }
            None => None,
        };

        let response = ChatCompletionResponse {
            id,
            object: "chat.completion".to_string(),
            created,
            model: request.model,
            choices: vec![Choice {
                index: 0,
                message: Message {
                    role: "assistant".to_string(),
                    content: content_buffer,
                    audio,
                },
                finish_reason: finish_reason.to_string(),
            }],
            usage: Usage {
                prompt_tokens,
                completion_tokens,
                total_tokens: prompt_tokens + completion_tokens,
            },
        };

        axum::response::Response::builder()
            .status(axum::http::StatusCode::OK)
            .header("Content-Type", "application/json")
            .body(axum::body::Body::from(
                serde_json::to_string(&response).unwrap(),
            ))
            .unwrap()
    }
}

/// Render a generated reply through an audio pipeline.
async fn speak_reply(
    pipeline: std::sync::Arc<tokio::sync::Mutex<crate::audio::pipeline::AudioPipeline>>,
    audio_parameter: &AudioParameter,
    reply: &str,
    seed: Option<u32>,
    id: &str,
    created: u64,
) -> Result<AudioContent, error::Error> {
    // Seconds of CPU-bound work; the async mutex is taken before entering the
    // blocking pool so queued requests do not pin its few threads. Encoding
    // runs there too: a full-length reply is ~650k samples to quantize and
    // ~1.8 MB to base64, which has no business on a reactor thread.
    let mut guard = pipeline.lock_owned().await;
    let spoken = reply.to_string();
    let as_wav = audio_parameter.format.as_deref().unwrap_or("wav") != "pcm";
    let data = tokio::task::spawn_blocking(move || {
        guard.generate(&spoken, None, seed).map(|output| {
            let encoded = if as_wav {
                crate::audio::pipeline::encode_wav(&output.samples, output.sample_rate)
            } else {
                crate::audio::pipeline::encode_pcm16(&output.samples)
            };
            encode_base64(&encoded)
        })
    })
    .await
    .map_err(|e| error::error!("Audio generation task failed: {}", e))?
    .map_err(|e| error::error!("{}", e))?;

    Ok(AudioContent {
        id: format!("audio-{id}"),
        data,
        // Nothing is retained server-side, so the audio is never retrievable by
        // id; OpenAI requires the field regardless, and its clients reject a
        // payload without it. Carries the request's `created`, so it reads as
        // already expired -- which it is, the data being inline in `data`.
        expires_at: created,
        transcript: reply.to_string(),
    })
}

/// Standard base64 with padding (RFC 4648 section 4).
fn encode_base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut encoded = String::with_capacity(bytes.len().div_ceil(3) * 4);
    for chunk in bytes.chunks(3) {
        let mut group = [0u8; 3];
        group[..chunk.len()].copy_from_slice(chunk);
        let bits = u32::from(group[0]) << 16 | u32::from(group[1]) << 8 | u32::from(group[2]);
        for index in 0..4 {
            // Each input byte contributes to two output characters, so a
            // 1-byte tail fills 2 of them and a 2-byte tail 3; the rest is pad.
            if index <= chunk.len() {
                encoded.push(ALPHABET[(bits >> (18 - 6 * index) & 0x3F) as usize] as char);
            } else {
                encoded.push('=');
            }
        }
    }
    encoded
}

pub(crate) fn openai_error_response(
    status: axum::http::StatusCode,
    message: &str,
    error_type: &str,
) -> axum::response::Response<axum::body::Body> {
    let error_response = serde_json::json!({
        "error": {
            "message": message,
            "type": error_type,
            "param": null,
            "code": null
        }
    });

    axum::response::Response::builder()
        .status(status)
        .header("Content-Type", "application/json")
        .body(axum::body::Body::from(
            serde_json::to_string(&error_response).unwrap(),
        ))
        .unwrap()
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Padding is where a hand-rolled encoder goes wrong, and a corrupt tail
    /// would surface as audio that fails to decode rather than as an error.
    #[test]
    fn test_encode_base64_pads_every_tail_length() {
        // RFC 4648 section 10 test vectors.
        assert_eq!(encode_base64(b""), "");
        assert_eq!(encode_base64(b"f"), "Zg==");
        assert_eq!(encode_base64(b"fo"), "Zm8=");
        assert_eq!(encode_base64(b"foo"), "Zm9v");
        assert_eq!(encode_base64(b"foob"), "Zm9vYg==");
        assert_eq!(encode_base64(b"fooba"), "Zm9vYmE=");
        assert_eq!(encode_base64(b"foobar"), "Zm9vYmFy");
    }

    #[test]
    fn test_encode_base64_covers_the_alphabet() {
        // 0xFB 0xFF exercises the '+' and '/' end of the table.
        assert_eq!(encode_base64(&[0xFB, 0xFF, 0xBF]), "+/+/");
        assert_eq!(encode_base64(&[0x00, 0x00, 0x00]), "AAAA");
    }
}
