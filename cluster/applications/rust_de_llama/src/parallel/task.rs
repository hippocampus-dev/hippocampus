pub struct Task {
    pub id: String,
    pub request: crate::handler::chat_completions::GenerateRequest,
    pub response_tx: tokio::sync::mpsc::Sender<Result<TaskResponse, error::Error>>,
    pub stop: Option<Vec<String>>,
}

pub enum TaskResponse {
    Token(String),
    Complete {
        prompt_tokens: u32,
        completion_tokens: u32,
    },
}
