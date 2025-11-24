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
    pub top_p: Option<f32>,
    pub max_tokens: Option<u32>,
    pub stream: Option<bool>,
    pub frequency_penalty: Option<f32>,
    pub presence_penalty: Option<f32>,
    pub seed: Option<u32>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Message {
    pub role: String,
    pub content: String,
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
    let model_name = &request.model;
    let processor = match state.llama_backend.get_or_load_model(model_name).await {
        Ok(p) => p,
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

    let (tx, mut rx) =
        tokio::sync::mpsc::channel::<Result<crate::parallel::TaskResponse, error::Error>>(100);

    let stop_sequences = model_config.stop_sequences.clone();

    let task = crate::parallel::Task {
        id: id.clone(),
        request: GenerateRequest {
            prompt,
            max_tokens: request.max_tokens,
            temperature: request.temperature,
            top_k: None,
            top_p: request.top_p,
            frequency_penalty: request.frequency_penalty,
            presence_penalty: request.presence_penalty,
            seed: request.seed,
        },
        response_tx: tx,
        stop: stop_sequences,
    };

    if let Err(e) = processor.submit_task(task) {
        let error_response = serde_json::json!({
            "error": {
                "message": format!("Failed to submit task: {e}"),
                "type": "internal_error",
                "param": null,
                "code": null
            }
        });

        return axum::response::Response::builder()
            .status(axum::http::StatusCode::INTERNAL_SERVER_ERROR)
            .header("Content-Type", "application/json")
            .body(axum::body::Body::from(
                serde_json::to_string(&error_response).unwrap(),
            ))
            .unwrap();
    }

    if stream {
        let id_for_stream = id.clone();
        let model = request.model.clone();
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
            };

            let data = format!("data: {}\n\n", serde_json::to_string(&initial_chunk).unwrap());
            yield Ok::<_, std::convert::Infallible>(data);

            while let Some(response) = rx.recv().await {
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
                        };

                        let data = format!("data: {}\n\n", serde_json::to_string(&chunk).unwrap());
                        yield Ok(data);
                    }
                    Ok(crate::parallel::TaskResponse::Complete { prompt_tokens: pt, completion_tokens: ct }) => {
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
                                finish_reason: Some("stop".to_string()),
                            }],
                        };

                        let data = format!("data: {}\n\n", serde_json::to_string(&final_chunk).unwrap());
                        yield Ok(data);

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

        let stream_body = axum::body::Body::wrap_stream(stream);

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

        while let Some(response) = rx.recv().await {
            match response {
                Ok(crate::parallel::TaskResponse::Token(token)) => {
                    content_buffer.push_str(&token);
                }
                Ok(crate::parallel::TaskResponse::Complete {
                    prompt_tokens: pt,
                    completion_tokens: ct,
                }) => {
                    prompt_tokens = pt;
                    completion_tokens = ct;
                    break;
                }
                Err(e) => {
                    let error_response = serde_json::json!({
                        "error": {
                            "message": format!("Generation error: {e}"),
                            "type": "generation_error",
                            "param": null,
                            "code": null
                        }
                    });

                    return axum::response::Response::builder()
                        .status(axum::http::StatusCode::INTERNAL_SERVER_ERROR)
                        .header("Content-Type", "application/json")
                        .body(axum::body::Body::from(
                            serde_json::to_string(&error_response).unwrap(),
                        ))
                        .unwrap();
                }
            }
        }

        state.processed_tokens_counter.add(
            &opentelemetry::Context::current(),
            (prompt_tokens + completion_tokens) as u64,
            &[],
        );

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
                },
                finish_reason: "stop".to_string(),
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
