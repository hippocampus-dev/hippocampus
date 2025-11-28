use serde::Deserialize;

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
    pub stop_sequences: Option<Vec<String>>,
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
