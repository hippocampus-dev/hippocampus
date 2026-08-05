/// A loaded model and the handles the manager needs to evict it: the shutdown
/// watch its processing loop selects on when idle, and a monotonically assigned
/// access tick giving the LRU order (larger = more recently used).
struct CachedModel {
    processor: std::sync::Arc<crate::parallel::ParallelProcessor>,
    shutdown_tx: tokio::sync::watch::Sender<bool>,
    /// Resolves once the processing loop has dropped its context, freeing the
    /// model's KV memory; awaited on eviction before loading the replacement so
    /// peak residency stays within the cap.
    teardown_rx: tokio::sync::oneshot::Receiver<()>,
    last_used: std::sync::atomic::AtomicU64,
}

type ModelCache = std::collections::HashMap<String, CachedModel>;

/// Why a model or an audio pipeline could not be handed back. The variants
/// exist because the handler cannot answer honestly without them: "all are
/// busy" and "no such model" are both failures to produce one, but only the
/// second means the client asked for something that does not exist, and
/// reporting the first as 404 tells a client its configured voice is gone when
/// the same request would succeed once a slot frees.
pub enum LoadError {
    /// Nothing by that name is configured, or its file is absent.
    NotFound(error::Error),
    /// Every resident entry is in use, so none can be evicted to make room.
    AtCapacity(error::Error),
    /// It is there, but it could not be brought up.
    Failed(error::Error),
}

impl std::fmt::Display for LoadError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            LoadError::NotFound(e) | LoadError::AtCapacity(e) | LoadError::Failed(e) => {
                write!(formatter, "{e}")
            }
        }
    }
}

/// A loaded audio pipeline and its LRU tick. There is no shutdown handshake as
/// there is for a chat model: a pipeline owns no processing loop, so dropping
/// the last `Arc` frees it. An in-flight request holds one of its own, which is
/// what makes `Arc::strong_count` the audio path's `active_slots`.
struct CachedAudioPipeline {
    pipeline: std::sync::Arc<tokio::sync::Mutex<crate::audio::pipeline::AudioPipeline>>,
    last_used: std::sync::atomic::AtomicU64,
}

type AudioPipelineCache = std::collections::HashMap<String, CachedAudioPipeline>;

pub struct ModelManager {
    models: std::sync::Arc<tokio::sync::RwLock<ModelCache>>,
    /// Lazily loaded audio pipelines, evicted least-recently-used against
    /// `max_loaded_models` as chat models are, but counted separately: see
    /// `get_or_load_audio_pipeline`.
    audio_pipelines: tokio::sync::RwLock<AudioPipelineCache>,
    model_directory: String,
    n_parallel: usize,
    n_ctx: i32,
    n_batch: i32,
    n_ubatch: i32,
    /// Cap on resident models; `None` keeps the unbounded default.
    max_loaded_models: Option<usize>,
    /// Monotonic clock stamped onto `CachedModel::last_used` on every access, so
    /// LRU order needs no wall-clock time.
    access_counter: std::sync::atomic::AtomicU64,
    models_config: Option<crate::config::ModelsConfig>,
    meter: opentelemetry::metrics::Meter,
}

impl ModelManager {
    pub fn new(
        model_directory: String,
        n_parallel: usize,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
        max_loaded_models: Option<usize>,
        meter: opentelemetry::metrics::Meter,
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
            audio_pipelines: tokio::sync::RwLock::default(),
            model_directory,
            n_parallel,
            n_ctx,
            n_batch,
            n_ubatch,
            max_loaded_models,
            access_counter: std::sync::atomic::AtomicU64::new(0),
            models_config,
            meter,
        }
    }

    /// Next monotonic tick for `last_used`; the smallest tick is the LRU victim.
    fn next_access_tick(&self) -> u64 {
        self.access_counter
            .fetch_add(1, std::sync::atomic::Ordering::Relaxed)
    }

    pub async fn get_or_load_model(
        &self,
        model_name: &str,
    ) -> Result<std::sync::Arc<crate::parallel::ParallelProcessor>, LoadError> {
        let mut components = std::path::Path::new(model_name).components();
        let is_single_normal = matches!(components.next(), Some(std::path::Component::Normal(_)))
            && components.next().is_none();
        if !is_single_normal {
            return Err(LoadError::NotFound(error::error!(
                "Model file '{}' not found in {}",
                model_name,
                self.model_directory
            )));
        }

        if let Some(entry) = self.models.read().await.get(model_name) {
            entry.last_used.store(
                self.next_access_tick(),
                std::sync::atomic::Ordering::Relaxed,
            );
            return Ok(entry.processor.clone());
        }

        let mut models = self.models.write().await;
        if let Some(entry) = models.get(model_name) {
            entry.last_used.store(
                self.next_access_tick(),
                std::sync::atomic::Ordering::Relaxed,
            );
            return Ok(entry.processor.clone());
        }

        let model_path = std::path::Path::new(&self.model_directory).join(model_name);

        if !model_path.exists() {
            return Err(LoadError::NotFound(error::error!(
                "Model file '{}' not found in {}",
                model_name,
                self.model_directory
            )));
        }

        // Make room under the resident-model cap before loading. Evict the
        // least-recently-used model with no in-flight generation, signalling its
        // processing loop to tear down (freeing its context and weights) at the
        // next idle point. If every resident model is busy, refuse the load
        // rather than interrupt an in-flight generation.
        if let Some(max) = self.max_loaded_models {
            while models.len() >= max {
                let victim = models
                    .iter()
                    .filter(|(_, entry)| entry.processor.active_slots() == 0)
                    .min_by_key(|(_, entry)| {
                        entry.last_used.load(std::sync::atomic::Ordering::Relaxed)
                    })
                    .map(|(name, _)| name.clone());
                match victim {
                    Some(name) => {
                        if let Some(entry) = models.remove(&name) {
                            // Block until the loop has torn down and freed its KV
                            // before loading the replacement. A dropped sender
                            // (loop already gone) resolves the await too.
                            let CachedModel {
                                processor,
                                shutdown_tx,
                                teardown_rx,
                                ..
                            } = entry;
                            drop(processor);
                            let _ = shutdown_tx.send(true);
                            let _ = teardown_rx.await;
                            tracing::info!("Evicted least-recently-used model: {}", name);
                        }
                    }
                    None => {
                        return Err(LoadError::AtCapacity(error::error!(
                            "Cannot load model '{}': {} models already loaded (max {}) and all are busy",
                            model_name,
                            models.len(),
                            max
                        )));
                    }
                }
            }
        }

        tracing::info!("Loading model: {}", model_name);

        let model_config = self
            .models_config
            .as_ref()
            .and_then(|config| config.get_model_config(model_name));

        let n_gpu_layers = model_config.and_then(|c| c.n_gpu_layers);
        let n_cpu_moe = model_config.and_then(|c| c.n_cpu_moe);

        // Context/model tuning knobs. Absent means llama.cpp's defaults: KV cache
        // in F16, KV offloaded to the compute device, weights not pinned.
        let type_k = model_config
            .and_then(|c| c.type_k)
            .map(|t| t.to_ggml_type())
            .unwrap_or_else(|| crate::config::KvCacheType::F16.to_ggml_type());
        let type_v_config = model_config.and_then(|c| c.type_v);
        // A quantized V cache needs flash attention, which this path (flash_attn_type
        // at AUTO) cannot guarantee. Reject early rather than fail later inside the
        // processor thread; the error below carries the actionable fix.
        if let Some(crate::config::KvCacheType::Q8_0) = type_v_config {
            return Err(LoadError::Failed(error::error!(
                "Model '{}' configures a quantized type_v (q8_0) V cache, which requires flash attention; this serving path leaves flash_attn_type at AUTO and cannot guarantee it. Remove type_v in models.toml and quantize type_k instead (type_k has no flash-attention requirement)",
                model_name
            )));
        }
        let type_v = type_v_config
            .map(|t| t.to_ggml_type())
            .unwrap_or_else(|| crate::config::KvCacheType::F16.to_ggml_type());
        let offload_kqv = model_config.and_then(|c| c.offload_kqv).unwrap_or(true);
        let use_mlock = model_config.and_then(|c| c.use_mlock).unwrap_or(false);
        let prompt_cache = model_config.and_then(|c| c.prompt_cache).unwrap_or(false);
        let speculation = model_config
            .and_then(|c| c.speculation.as_ref())
            .filter(|speculation| speculation.enabled)
            .map(|speculation| (speculation.ngram, speculation.max_draft));

        let default_threads = default_thread_count();
        let n_threads = model_config
            .and_then(|c| c.n_threads)
            .unwrap_or(default_threads);
        let n_threads_batch = model_config
            .and_then(|c| c.n_threads_batch)
            .unwrap_or(default_threads);

        let model = rust_de_llama::LlamaModel::load_from_file(
            &model_path,
            n_gpu_layers,
            n_cpu_moe,
            use_mlock,
        )
        .map_err(|e| LoadError::Failed(error::error!("Failed to load model: {}", e)))?;

        let model_n_ctx_train = model.n_ctx_train();
        let model_arc = std::sync::Arc::new(model);

        // Stage-2 draft model for speculative decoding: a small same-family model
        // whose greedy proposals the target verifies. Loaded after the target so
        // its vocabulary can be checked for compatibility, then handed to the
        // processor to replace prompt-lookup as the proposal source.
        let draft_model = match model_config
            .and_then(|c| c.speculation.as_ref())
            .filter(|speculation| speculation.enabled)
            .and_then(|speculation| speculation.draft_model.as_deref())
        {
            Some(draft_name) => {
                let draft_path = std::path::Path::new(&self.model_directory).join(draft_name);
                if !draft_path.exists() {
                    // The model the client asked for is there; its speculation
                    // config names a draft that is not, which is a failure to
                    // bring that model up rather than a missing model.
                    return Err(LoadError::Failed(error::error!(
                        "Draft model '{}' not found in {}",
                        draft_name,
                        self.model_directory
                    )));
                }
                let draft = rust_de_llama::LlamaModel::load_from_file(
                    &draft_path,
                    n_gpu_layers,
                    None,
                    use_mlock,
                )
                .map_err(|e| {
                    LoadError::Failed(error::error!(
                        "Failed to load draft model '{}': {}",
                        draft_name,
                        e
                    ))
                })?;
                if draft.n_vocab() != model_arc.n_vocab() {
                    return Err(LoadError::Failed(error::error!(
                        "Draft model '{}' vocab size {} does not match target model '{}' vocab size {}; draft-model speculation requires a same-family draft",
                        draft_name,
                        draft.n_vocab(),
                        model_name,
                        model_arc.n_vocab()
                    )));
                }
                tracing::info!("Loaded draft model '{}' for '{}'", draft_name, model_name);
                Some(std::sync::Arc::new(draft))
            }
            None => None,
        };

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
            "Initializing model '{}' with: n_ctx={}, n_parallel={}, n_batch={}, n_ubatch={}, n_cpu_moe={}, n_threads={}, n_threads_batch={}",
            model_name,
            n_ctx,
            n_parallel,
            n_batch,
            n_ubatch,
            n_cpu_moe.unwrap_or(0),
            n_threads,
            n_threads_batch
        );

        let (processor, task_rx) = crate::parallel::ParallelProcessor::new(
            model_arc,
            n_parallel,
            n_ctx,
            n_batch,
            n_ubatch,
            n_threads,
            n_threads_batch,
            type_k,
            type_v,
            offload_kqv,
            prompt_cache,
            speculation,
            draft_model,
            &self.meter,
            model_name,
        )
        .map_err(LoadError::Failed)?;

        let processor_arc = std::sync::Arc::new(processor);

        // Eviction signal for the loop's idle branch and its teardown-complete
        // reply; see `CachedModel` for how eviction uses them.
        let (shutdown_tx, shutdown_rx) = tokio::sync::watch::channel(false);
        let (teardown_tx, teardown_rx) = tokio::sync::oneshot::channel();

        std::thread::spawn({
            let processor = processor_arc.clone();
            move || {
                // Current-thread: the loop is one sequential task that owns the
                // context, so a multi-threaded runtime would add a worker per
                // logical core per loaded model with nothing to steal -- threads
                // that then contend with ggml's own pool for the same cores.
                tokio::runtime::Builder::new_current_thread()
                    .enable_all()
                    .build()
                    .unwrap()
                    .block_on(processor.run_processing_loop(task_rx, shutdown_rx, teardown_tx));
            }
        });

        models.insert(
            model_name.to_string(),
            CachedModel {
                processor: processor_arc.clone(),
                shutdown_tx,
                teardown_rx,
                last_used: std::sync::atomic::AtomicU64::new(self.next_access_tick()),
            },
        );

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

    /// Lazily load an audio pipeline by its `[audio_pipelines]` name, reporting
    /// why it could not through `LoadError` as the chat path does.
    #[tracing::instrument(skip(self))]
    pub async fn get_or_load_audio_pipeline(
        &self,
        pipeline_name: &str,
    ) -> Result<std::sync::Arc<tokio::sync::Mutex<crate::audio::pipeline::AudioPipeline>>, LoadError>
    {
        let Some(config) = self
            .models_config
            .as_ref()
            .and_then(|config| config.get_audio_pipeline_config(pipeline_name))
            .cloned()
        else {
            return Err(LoadError::NotFound(error::error!(
                "Audio pipeline '{}' is not configured",
                pipeline_name
            )));
        };

        if let Some(cached) = self.audio_pipelines.read().await.get(pipeline_name) {
            cached.last_used.store(
                self.next_access_tick(),
                std::sync::atomic::Ordering::Relaxed,
            );
            return Ok(cached.pipeline.clone());
        }

        // Resolve the shared thinker before taking the audio cache's write
        // lock, and let the read guard go at the end of this statement. The two
        // locks are only ever taken in this order so neither ordering
        // deadlocks, but holding the audio write lock across this read parks
        // every audio request -- cache hits included -- behind whatever chat
        // model load happens to hold `models` for writing.
        let loaded_thinker = match &config.thinker_model {
            Some(thinker_model) => self
                .models
                .read()
                .await
                .get(thinker_model)
                .map(|cached| cached.processor.model()),
            None => None,
        };

        let mut pipelines = self.audio_pipelines.write().await;
        if let Some(cached) = pipelines.get(pipeline_name) {
            cached.last_used.store(
                self.next_access_tick(),
                std::sync::atomic::Ordering::Relaxed,
            );
            return Ok(cached.pipeline.clone());
        }

        // Make room under the resident cap, as the chat path does. The cap is
        // counted per cache rather than shared with `models`: the two are
        // guarded by different locks, and a pipeline holds up to three GGUFs
        // against a chat model's one, so a shared count would not measure the
        // residency it is meant to bound anyway.
        if let Some(max) = self.max_loaded_models {
            while pipelines.len() >= max {
                // Only the cache holding an Arc means no request is in flight;
                // a handler clones one for the length of its generation.
                let victim = pipelines
                    .iter()
                    .filter(|(_, entry)| std::sync::Arc::strong_count(&entry.pipeline) == 1)
                    .min_by_key(|(_, entry)| {
                        entry.last_used.load(std::sync::atomic::Ordering::Relaxed)
                    })
                    .map(|(name, _)| name.clone());
                match victim {
                    Some(name) => {
                        // Dropping the last Arc frees the models; there is no
                        // loop to wind down first.
                        pipelines.remove(&name);
                        tracing::info!("Evicted least-recently-used audio pipeline: {}", name);
                    }
                    None => {
                        return Err(LoadError::AtCapacity(error::error!(
                            "Cannot load audio pipeline '{}': {} pipelines already loaded (max {}) and all are busy",
                            pipeline_name,
                            pipelines.len(),
                            max
                        )));
                    }
                }
            }
        }

        // `loaded_thinker` reuses the chat path's weights when that GGUF is
        // already resident, so a thinker serving both is not loaded twice. Only
        // reused when already there: loading it here would drag in a whole
        // generation processor the audio path has no use for. Holding the Arc
        // keeps the weights alive past eviction of the chat entry, which is the
        // point -- but it also means evicting that entry no longer frees them.
        tracing::info!("Loading audio pipeline: {}", pipeline_name);
        let pipeline = crate::audio::pipeline::AudioPipeline::load(
            &self.model_directory,
            pipeline_name,
            &config,
            default_thread_count(),
            &self.meter,
            loaded_thinker,
        )
        .map_err(LoadError::Failed)?;
        let pipeline = std::sync::Arc::new(tokio::sync::Mutex::new(pipeline));
        pipelines.insert(
            pipeline_name.to_string(),
            CachedAudioPipeline {
                pipeline: pipeline.clone(),
                last_used: std::sync::atomic::AtomicU64::new(self.next_access_tick()),
            },
        );
        Ok(pipeline)
    }
}

/// Default worker-thread count when a model does not configure `n_threads`.
///
/// ggml's kernels run under OpenMP with an active spin-wait at barriers, so
/// oversubscribing SMT siblings (using logical instead of physical cores)
/// collapses decode throughput -- on a 16-core / 32-thread host, decode drops
/// from ~170 to ~1 tok/s. llama.cpp defaults to physical cores for the same
/// reason. Cap the physical-core count by `available_parallelism` so a
/// CPU-limited cgroup (which `available_parallelism` reflects but /proc/cpuinfo
/// does not) is still respected; fall back to the logical count when the
/// physical count cannot be determined.
fn default_thread_count() -> i32 {
    let logical = std::thread::available_parallelism()
        .map(|n| n.get())
        .unwrap_or(4);
    physical_core_count()
        .map(|physical| physical.min(logical))
        .unwrap_or(logical) as i32
}

/// Physical CPU core count from `/proc/cpuinfo`, counted as the number of unique
/// `(physical id, core id)` pairs. Returns `None` when the file is unavailable
/// or lacks those fields (non-Linux, or a container without topology), so the
/// caller can fall back to the logical count.
fn physical_core_count() -> Option<usize> {
    let cpuinfo = std::fs::read_to_string("/proc/cpuinfo").ok()?;
    let mut cores: std::collections::HashSet<(String, String)> = std::collections::HashSet::new();
    let mut physical_id: Option<String> = None;
    let mut core_id: Option<String> = None;
    let field = |line: &str| line.split(':').nth(1).map(|value| value.trim().to_string());
    for line in cpuinfo.lines() {
        if line.is_empty() {
            if let (Some(physical), Some(core)) = (physical_id.take(), core_id.take()) {
                cores.insert((physical, core));
            }
        } else if line.starts_with("physical id") {
            physical_id = field(line);
        } else if line.starts_with("core id") {
            core_id = field(line);
        }
    }
    // The last record is not followed by a blank line.
    if let (Some(physical), Some(core)) = (physical_id, core_id) {
        cores.insert((physical, core));
    }
    (!cores.is_empty()).then_some(cores.len())
}
