#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Modality {
    Audio,
    Text,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct InputAudioTranscription {
    pub language: String,
    pub model: String,
    pub prompt: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct TurnDetection {
    pub create_response: bool,
    pub interrupt_response: bool,
    pub prefix_padding_ms: u32,
    pub silence_duration_ms: u32,
    pub threshold: f32,
    #[serde(rename = "type")]
    pub r#type: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct PropertyValue {
    #[serde(rename = "type")]
    pub r#type: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct ToolParameter {
    #[serde(rename = "type")]
    pub r#type: String,
    pub properties: std::collections::HashMap<String, PropertyValue>,
    pub required: Vec<String>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Tool {
    pub description: String,
    pub name: String,
    pub parameters: ToolParameter,
    #[serde(rename = "type")]
    pub r#type: String,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct Session {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_audio_format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_audio_transcription: Option<InputAudioTranscription>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub instructions: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_response_output_tokens: Option<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub modalities: Vec<Modality>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_audio_format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub tools: Vec<Tool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub turn_detection: Option<TurnDetection>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(tag = "type")]
pub enum Event {
    #[serde(rename = "session.update")]
    SessionUpdate { session: Session },

    #[serde(rename = "input_audio_buffer.append")]
    InputAudioBufferAppend { audio: String },

    #[serde(rename = "response.text.delta")]
    ResponseTextDelta {
        response_id: String,
        item_id: String,
        output_index: u32,
        content_index: u32,
        delta: String,
    },

    #[serde(rename = "response.text.done")]
    ResponseTextDone {
        context_index: u32,
        event_id: String,
        item_id: String,
        output_index: u32,
        response_id: String,
        text: String,
    },
}
