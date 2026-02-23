#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Role {
    System,
    User,
    Assistant,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct MessageContentTypedImageUrl {
    pub url: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum TypedContent {
    Text {
        text: String,
    },
    ImageUrl {
        image_url: MessageContentTypedImageUrl,
    },
    Audio {
        id: String,
    },
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(untagged)]
pub enum Content {
    String(String),
    Typed(Vec<TypedContent>),
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct RequestMessage {
    pub role: Role,
    pub content: Content,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ReasoningEffort {
    #[serde(rename = "none")]
    Disabled,
    Minimal,
    Low,
    Medium,
    High,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Verbosity {
    Low,
    Medium,
    High,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ResponseFormat {
    Text,
    JsonObject,
    JsonSchema,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct CompletionRequestBody {
    pub model: String,
    pub messages: Vec<RequestMessage>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reasoning_effort: Option<ReasoningEffort>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub verbosity: Option<Verbosity>,
    pub stream: bool,
    pub n: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_format: Option<ResponseFormat>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Usage {
    pub prompt_tokens: i32,
    pub completion_tokens: i32,
    pub total_tokens: i32,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct ResponseMessage {
    pub role: Role,
    pub content: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Choice {
    pub message: ResponseMessage,
    pub finish_reason: String,
    pub index: i32,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct CompletionResponseBody {
    pub id: String,
    pub object: String,
    pub created: i32,
    pub model: String,
    pub usage: Usage,
    pub choices: Vec<Choice>,
}
