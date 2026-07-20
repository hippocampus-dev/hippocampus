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
}

impl ModelsConfig {
    pub fn load_from_file(path: &std::path::Path) -> Result<Self, Box<dyn std::error::Error>> {
        let contents = std::fs::read_to_string(path)?;
        let config: ModelsConfig = toml::from_str(&contents)?;
        Ok(config)
    }

    pub fn get_model_config(&self, model_name: &str) -> Option<&ModelConfig> {
        self.models.get(model_name)
    }
}
