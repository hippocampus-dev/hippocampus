use serde::Deserialize;

/// KV cache element precision (`type_k` / `type_v`). `q8_0` halves KV memory --
/// convertible into a larger `n_ctx` or more parallel slots -- at a small,
/// for K near-lossless, quality cost.
#[derive(Debug, Clone, Copy, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum KvCacheType {
    F16,
    Q8_0,
}

impl KvCacheType {
    /// `ggml_type` discriminant expected by `llama_context_params.type_k`/`type_v`.
    pub fn to_ggml_type(self) -> i32 {
        match self {
            KvCacheType::F16 => 1,  // GGML_TYPE_F16
            KvCacheType::Q8_0 => 8, // GGML_TYPE_Q8_0
        }
    }
}

/// Prompt-lookup (n-gram) speculative decoding. Off by default: with greedy
/// sampling acceptance-by-equality preserves the output exactly, but the feature
/// still ships disabled because it only pays off on extractive/copy-heavy
/// workloads and its acceptance semantics are documented as a tradeoff.
#[derive(Debug, Clone, Deserialize)]
pub struct SpeculationConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default = "SpeculationConfig::default_ngram")]
    pub ngram: usize,
    #[serde(default = "SpeculationConfig::default_max_draft")]
    pub max_draft: usize,
    /// GGUF filename (in the model directory) of a small draft model of the same
    /// family. When set, a draft-model proposer replaces prompt-lookup: the draft
    /// model greedily proposes `max_draft` tokens for the target to verify, which
    /// speeds up general workloads rather than only extractive ones. Off (None)
    /// keeps prompt-lookup speculation.
    #[serde(default)]
    pub draft_model: Option<String>,
}

impl SpeculationConfig {
    fn default_ngram() -> usize {
        2
    }

    fn default_max_draft() -> usize {
        4
    }
}

/// Thinker-Talker audio pipeline: the talker autoregressively emits audio-codec
/// tokens for the text, and a WavTokenizer GGUF decodes them to a 24 kHz
/// waveform. The text reaches the talker as its own tokens, or -- when a thinker
/// and a projection (`notebooks/thinker-talker-projection.ipynb`) are configured
/// -- as that thinker's projected per-token hidden states. Requested through
/// `/v1/audio/speech` with the pipeline name as `model`, or through
/// `/v1/chat/completions` as `audio.voice` to speak a chat model's reply.
#[derive(Debug, Clone, Deserialize)]
pub struct AudioPipelineConfig {
    /// GGUF file name (in the model directory) of the thinker LLM. Configured
    /// together with `projection_model`: the projection maps this thinker's
    /// hidden states, so neither is meaningful without the other.
    #[serde(default)]
    pub thinker_model: Option<String>,
    /// GGUF file name of the talker: a causal LM whose vocabulary encodes the
    /// decoder's codes as the token run `<|0|>`..`<|4095|>` (OuteTTS-style).
    pub talker_model: String,
    /// GGUF file name of the trained projection layer. Omit it to let the
    /// talker read the text itself, which needs no training but confines the
    /// pipeline to what the talker's own tokenizer can say; set it to route the
    /// thinker's hidden states through the projection instead.
    #[serde(default)]
    pub projection_model: Option<String>,
    /// GGUF file name of the WavTokenizer decoder (`wavtokenizer-dec` arch).
    pub audio_decoder: String,
    /// Thinker context size; bounds the input length in tokens.
    #[serde(default = "AudioPipelineConfig::default_n_ctx")]
    pub n_ctx: i32,
    /// Talker context size. It must hold the speaker scaffolding that
    /// conditions the voice, the projected prefix and every generated code, so
    /// it is sized independently of `n_ctx`.
    #[serde(default = "AudioPipelineConfig::default_talker_n_ctx")]
    pub talker_n_ctx: i32,
    /// Audio-code sampling temperature; overridable per request.
    #[serde(default = "AudioPipelineConfig::default_temperature")]
    pub temperature: f32,
}

impl AudioPipelineConfig {
    fn default_n_ctx() -> i32 {
        2048
    }

    fn default_talker_n_ctx() -> i32 {
        4096
    }

    fn default_temperature() -> f32 {
        0.7
    }
}

#[derive(Debug, Clone, Deserialize)]
pub struct PromptFormat {
    #[serde(default)]
    pub user_prefix: Option<String>,
    #[serde(default)]
    pub user_suffix: Option<String>,
    #[serde(default)]
    pub assistant_prefix: Option<String>,
    #[serde(default)]
    pub assistant_suffix: Option<String>,
    #[serde(default)]
    pub system_prefix: Option<String>,
    #[serde(default)]
    pub system_suffix: Option<String>,
    #[serde(default)]
    pub add_generation_prompt: Option<String>,
}

impl Default for PromptFormat {
    fn default() -> Self {
        Self {
            user_prefix: None,
            user_suffix: None,
            assistant_prefix: None,
            assistant_suffix: None,
            system_prefix: None,
            system_suffix: None,
            add_generation_prompt: None,
        }
    }
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct ModelConfig {
    #[serde(default)]
    pub n_ctx: Option<i32>,
    #[serde(default)]
    pub n_parallel: Option<usize>,
    #[serde(default)]
    pub n_batch: Option<i32>,
    #[serde(default)]
    pub n_ubatch: Option<i32>,
    #[serde(default)]
    pub n_gpu_layers: Option<i32>,
    #[serde(default)]
    pub n_cpu_moe: Option<i32>,
    #[serde(default)]
    pub n_threads: Option<i32>,
    #[serde(default)]
    pub n_threads_batch: Option<i32>,
    #[serde(default)]
    pub request_idle_timeout_seconds: Option<u64>,
    #[serde(default)]
    pub max_prompt_tokens: Option<i32>,
    #[serde(default)]
    pub stop_sequences: Option<Vec<String>>,
    #[serde(default)]
    pub prompt_cache: Option<bool>,
    #[serde(default)]
    pub speculation: Option<SpeculationConfig>,
    #[serde(default)]
    pub type_k: Option<KvCacheType>,
    #[serde(default)]
    pub type_v: Option<KvCacheType>,
    #[serde(default)]
    pub offload_kqv: Option<bool>,
    #[serde(default)]
    pub use_mlock: Option<bool>,
    #[serde(default)]
    pub prompt_format: PromptFormat,
}

#[derive(Debug, Deserialize)]
pub struct ModelsConfig {
    pub models: std::collections::HashMap<String, ModelConfig>,
    /// Thinker-Talker audio pipelines keyed by the name clients pass as
    /// `model` on `/v1/audio/speech`.
    #[serde(default)]
    pub audio_pipelines: std::collections::HashMap<String, AudioPipelineConfig>,
}

impl ModelsConfig {
    pub fn load_from_file(path: &std::path::Path) -> Result<Self, Box<dyn std::error::Error>> {
        let contents = std::fs::read_to_string(path)?;
        let config: ModelsConfig = toml::from_str(&contents)?;
        config.validate()?;
        Ok(config)
    }

    fn validate(&self) -> Result<(), Box<dyn std::error::Error>> {
        for (name, audio_pipeline) in &self.audio_pipelines {
            // An audio pipeline name equal to a `[models]` key would shadow the
            // model at request routing; refuse the ambiguity at load time.
            if self.models.contains_key(name) {
                return Err(format!(
                    "Audio pipeline '{name}' collides with a [models] entry of the same name"
                )
                .into());
            }
            // Either half alone would silently do nothing: a thinker whose
            // hidden states nothing maps, or a projection with no hidden states
            // to map.
            if audio_pipeline.thinker_model.is_some() != audio_pipeline.projection_model.is_some() {
                return Err(format!(
                    "Audio pipeline '{name}' sets only one of thinker_model and projection_model; set both to project the thinker's hidden states, or neither to let the talker read the text"
                )
                .into());
            }
        }
        Ok(())
    }

    pub fn get_model_config(&self, model_name: &str) -> Option<&ModelConfig> {
        self.models.get(model_name)
    }

    pub fn get_audio_pipeline_config(&self, pipeline_name: &str) -> Option<&AudioPipelineConfig> {
        self.audio_pipelines.get(pipeline_name)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_audio_pipeline_defaults_and_collision() {
        let config: ModelsConfig = toml::from_str(
            r#"
            [models."a.gguf"]

            [audio_pipelines."voice"]
            thinker_model = "a.gguf"
            talker_model = "t.gguf"
            projection_model = "p.gguf"
            audio_decoder = "w.gguf"
            "#,
        )
        .unwrap();
        let audio_pipeline = config.get_audio_pipeline_config("voice").unwrap();
        assert_eq!(audio_pipeline.n_ctx, 2048);
        assert_eq!(audio_pipeline.talker_n_ctx, 4096);
        assert_eq!(audio_pipeline.temperature, 0.7);
        assert!(config.validate().is_ok());

        let config: ModelsConfig = toml::from_str(
            r#"
            [models."voice"]

            [audio_pipelines."voice"]
            thinker_model = "a.gguf"
            talker_model = "t.gguf"
            projection_model = "p.gguf"
            audio_decoder = "w.gguf"
            "#,
        )
        .unwrap();
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_no_audio_pipelines_section() {
        let config: ModelsConfig = toml::from_str(
            r#"
            [models."a.gguf"]
            n_ctx = 4096
            "#,
        )
        .unwrap();
        assert!(config.get_audio_pipeline_config("missing").is_none());
        assert!(config.validate().is_ok());
    }
}
