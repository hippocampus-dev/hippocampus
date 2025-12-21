type ModelCache =
    std::collections::HashMap<String, std::sync::Arc<crate::parallel::ParallelProcessor>>;

pub struct ModelManager {
    models: std::sync::Arc<tokio::sync::RwLock<ModelCache>>,
    model_directory: String,
    n_parallel: usize,
    n_ctx: i32,
    n_batch: i32,
    n_ubatch: i32,
    models_config: Option<crate::config::ModelsConfig>,
}

impl ModelManager {
    pub fn new(
        model_directory: String,
        n_parallel: usize,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
    ) -> Self {
        let config_path = std::path::Path::new(&model_directory).join("models.toml");
        let models_config = if config_path.exists() {
            match crate::config::ModelsConfig::load_from_file(&config_path) {
                Ok(config) => {
                    tracing::info!(
                        "Loaded models configuration from: {}",
                        config_path.display()
                    );
                    Some(config)
                }
                Err(e) => {
                    tracing::warn!("Failed to load models.toml: {}, using default settings", e);
                    None
                }
            }
        } else {
            tracing::info!("No models.toml found, using default settings for all models");
            None
        };

        Self {
            models: std::sync::Arc::default(),
            model_directory,
            n_parallel,
            n_ctx,
            n_batch,
            n_ubatch,
            models_config,
        }
    }

    pub async fn get_or_load_model(
        &self,
        model_name: &str,
    ) -> Result<std::sync::Arc<crate::parallel::ParallelProcessor>, error::Error> {
        if let Some(processor) = self.models.read().await.get(model_name) {
            return Ok(processor.clone());
        }

        let mut models = self.models.write().await;
        if let Some(processor) = models.get(model_name) {
            return Ok(processor.clone());
        }

        let model_path = std::path::Path::new(&self.model_directory).join(model_name);

        if !model_path.exists() {
            return Err(error::error!(
                "Model file '{}' not found in {}",
                model_name,
                self.model_directory
            ));
        }

        tracing::info!("Loading model: {}", model_name);

        let model_config = self.models_config
            .as_ref()
            .and_then(|config| config.get_model_config(model_name));

        let n_gpu_layers = model_config.and_then(|c| c.n_gpu_layers);

        let model = rust_de_llama::LlamaModel::load_from_file(&model_path, n_gpu_layers)
            .map_err(|e| error::error!("Failed to load model: {}", e))?;

        let model_n_ctx_train = model.n_ctx_train();
        let model_arc = std::sync::Arc::new(model);

        let (n_parallel, n_ctx, n_batch, n_ubatch) = if let Some(ref config) = self.models_config {
            if let Some(model_config) = config.get_model_config(model_name) {
                let configured_n_ctx = model_config.n_ctx.unwrap_or(self.n_ctx);
                (
                    model_config.n_parallel.unwrap_or(self.n_parallel),
                    if configured_n_ctx == 0 {
                        model_n_ctx_train
                    } else {
                        configured_n_ctx
                    },
                    model_config.n_batch.unwrap_or(self.n_batch),
                    model_config.n_ubatch.unwrap_or(self.n_ubatch),
                )
            } else {
                (
                    self.n_parallel,
                    if self.n_ctx == 0 {
                        model_n_ctx_train
                    } else {
                        self.n_ctx
                    },
                    self.n_batch,
                    self.n_ubatch,
                )
            }
        } else {
            (
                self.n_parallel,
                if self.n_ctx == 0 {
                    model_n_ctx_train
                } else {
                    self.n_ctx
                },
                self.n_batch,
                self.n_ubatch,
            )
        };

        tracing::info!(
            "Initializing model '{}' with: n_ctx={}, n_parallel={}, n_batch={}, n_ubatch={}",
            model_name,
            n_ctx,
            n_parallel,
            n_batch,
            n_ubatch
        );

        let (processor, task_rx) = crate::parallel::ParallelProcessor::new(
            model_arc, n_parallel, n_ctx, n_batch, n_ubatch,
        )?;

        let processor_arc = std::sync::Arc::new(processor);

        std::thread::spawn({
            let processor = processor_arc.clone();
            move || {
                tokio::runtime::Runtime::new()
                    .unwrap()
                    .block_on(processor.run_processing_loop(task_rx));
            }
        });

        models.insert(model_name.to_string(), processor_arc.clone());

        tracing::info!("Successfully loaded model: {}", model_name);

        Ok(processor_arc)
    }

    pub async fn get_model_config(&self, model_name: &str) -> crate::config::ModelConfig {
        self.models_config
            .as_ref()
            .and_then(|config| config.get_model_config(model_name))
            .cloned()
            .unwrap_or_default()
    }
}
