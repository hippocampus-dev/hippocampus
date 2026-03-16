#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Modality {
    Audio,
    Text,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SessionType {
    Realtime,
    Transcription,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct InputAudioTranscription {
    pub language: String,
    pub model: String,
    pub prompt: String,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct TurnDetection {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub create_response: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub interrupt_response: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prefix_padding_ms: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub silence_duration_ms: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub threshold: Option<f32>,
    #[serde(rename = "type")]
    pub r#type: String,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct AudioFormat {
    #[serde(rename = "type")]
    pub r#type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub rate: Option<u32>,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct AudioInput {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<AudioFormat>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub turn_detection: Option<TurnDetection>,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct AudioOutput {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<AudioFormat>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,
}

#[derive(Clone, Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct AudioConfiguration {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input: Option<AudioInput>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output: Option<AudioOutput>,
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
    #[serde(skip_serializing_if = "Option::is_none", rename = "type")]
    pub session_type: Option<SessionType>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio: Option<AudioConfiguration>,
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
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub output_modalities: Vec<Modality>,
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
    ResponseTextDelta { delta: String },

    #[serde(rename = "response.text.done")]
    ResponseTextDone {},

    #[serde(rename = "response.output_text.delta")]
    ResponseOutputTextDelta { delta: String },

    #[serde(rename = "response.output_text.done")]
    ResponseOutputTextDone {},
}
