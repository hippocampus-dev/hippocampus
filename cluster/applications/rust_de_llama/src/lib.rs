autocxx::include_cpp! {
    #include "wrapper.h"
    safety!(unsafe_ffi)

    // Backend functions
    generate!("llama_backend_init")
    generate!("llama_supports_mmap")
    generate!("llama_supports_mlock")
    generate!("llama_supports_gpu_offload")

    // Model functions
    generate!("llama_model_params")
    generate!("llama_model_default_params")
    generate!("llama_model_params_with_n_gpu_layers")
    generate!("llama_model_load_from_file")
    generate!("llama_model_free")
    generate!("llama_model_get_vocab")
    generate!("llama_model_n_ctx_train")

    // Context functions
    generate!("llama_context_default_params")
    generate!("llama_context_params_with_n_ctx")
    generate!("llama_context_params_with_n_ctx_and_n_ubatch")
    generate!("llama_init_from_model")
    generate!("llama_free")

    // Vocab and tokenization
    generate!("llama_tokenize")
    generate!("llama_vocab_is_eog")
    generate!("llama_token_to_piece")

    // Sampling
    generate!("llama_sampler_chain_default_params")
    generate!("llama_sampler_chain_init")
    generate!("llama_sampler_chain_add")
    generate!("llama_sampler_free")
    generate!("llama_sampler_sample")
    generate!("llama_sampler_reset")

    // Sampling methods
    generate!("llama_sampler_init_temp")
    generate!("llama_sampler_init_top_k")
    generate!("llama_sampler_init_top_p")
    generate!("llama_sampler_init_dist")
    generate!("llama_sampler_init_penalties")
}

pub use ffi::*;

#[repr(C)]
pub struct llama_batch {
    pub n_tokens: i32,
    pub token: *mut i32,
    pub embd: *mut f32,
    pub pos: *mut i32,
    pub n_seq_id: *mut i32,
    pub seq_id: *mut *mut i32,
    pub logits: *mut i8,
}

#[repr(C)]
#[derive(Debug, Copy, Clone, PartialEq, Eq)]
pub enum GgmlLogLevel {
    None = 0,
    Debug = 1,
    Info = 2,
    Warn = 3,
    Error = 4,
    Cont = 5,
}

pub type GgmlLogCallback = Option<
    extern "C" fn(
        level: GgmlLogLevel,
        text: *const std::os::raw::c_char,
        user_data: *mut std::os::raw::c_void,
    ),
>;

unsafe extern "C" {
    pub fn llama_decode(ctx: *mut llama_context, batch: llama_batch) -> i32;
    pub fn llama_log_set(log_callback: GgmlLogCallback, user_data: *mut std::os::raw::c_void);
}

static INIT: std::sync::Once = std::sync::Once::new();

extern "C" fn null_log_callback(
    _level: GgmlLogLevel,
    _text: *const std::os::raw::c_char,
    _user_data: *mut std::os::raw::c_void,
) {
}

pub fn ensure_backend_init() {
    INIT.call_once(|| {
        unsafe {
            llama_log_set(Some(null_log_callback), std::ptr::null_mut());
        }

        llama_backend_init();
    });
}

pub struct LlamaModel(*mut llama_model);

impl LlamaModel {
    pub fn load_from_file(
        path: &std::path::Path,
        n_gpu_layers: Option<i32>,
    ) -> Result<Self, String> {
        ensure_backend_init();

        #[cfg(feature = "cuda")]
        const DEFAULT_GPU_LAYERS: i32 = 999;
        #[cfg(not(feature = "cuda"))]
        const DEFAULT_GPU_LAYERS: i32 = 0;

        let n_gpu_layers = n_gpu_layers.unwrap_or(DEFAULT_GPU_LAYERS);

        let path_c = std::ffi::CString::new(path.to_str().ok_or("Invalid path")?)
            .map_err(|e| format!("Failed to create C string: {e}"))?;

        unsafe {
            let params = llama_model_params_with_n_gpu_layers(n_gpu_layers);
            let model = llama_model_load_from_file(
                path_c.as_ptr(),
                autocxx::WithinUniquePtr::within_unique_ptr(params),
            );

            if model.is_null() {
                return Err("Failed to load model".to_string());
            }

            Ok(Self(model))
        }
    }

    pub fn as_ptr(&self) -> *mut llama_model {
        self.0
    }

    pub fn get_vocab(&self) -> *const llama_vocab {
        unsafe { llama_model_get_vocab(self.0) }
    }

    pub fn n_ctx_train(&self) -> i32 {
        unsafe { llama_model_n_ctx_train(self.0) }
    }
}

impl Drop for LlamaModel {
    fn drop(&mut self) {
        if !self.0.is_null() {
            unsafe {
                llama_model_free(self.0);
            }
        }
    }
}

unsafe impl Send for LlamaModel {}
unsafe impl Sync for LlamaModel {}

pub struct LlamaContext(*mut llama_context);

impl LlamaContext {
    pub fn new(model: &LlamaModel, n_ctx: i32, n_ubatch: i32) -> Result<Self, String> {
        let params = llama_context_params_with_n_ctx_and_n_ubatch(n_ctx, n_ubatch);
        let context = unsafe {
            llama_init_from_model(
                model.as_ptr(),
                autocxx::WithinUniquePtr::within_unique_ptr(params),
            )
        };

        if context.is_null() {
            return Err("Failed to create context".to_string());
        }

        Ok(Self(context))
    }

    pub fn as_ptr(&self) -> *mut llama_context {
        self.0
    }

    pub fn decode(&self, batch: llama_batch) -> Result<(), String> {
        let result = unsafe { llama_decode(self.0, batch) };
        if result != 0 {
            return Err(format!("Decode failed with code: {result}"));
        }
        Ok(())
    }
}

impl Drop for LlamaContext {
    fn drop(&mut self) {
        if !self.0.is_null() {
            unsafe {
                llama_free(self.0);
            }
        }
    }
}

unsafe impl Send for LlamaContext {}
unsafe impl Sync for LlamaContext {}

pub struct LlamaSampler(*mut llama_sampler);

impl LlamaSampler {
    pub fn new() -> Result<Self, String> {
        let params = llama_sampler_chain_default_params();
        let sampler = llama_sampler_chain_init(autocxx::WithinUniquePtr::within_unique_ptr(params));

        if sampler.is_null() {
            return Err("Failed to create sampler chain".to_string());
        }

        Ok(Self(sampler))
    }

    pub fn reset(&self) {
        unsafe {
            llama_sampler_reset(self.0);
        }
    }

    pub unsafe fn chain_add(&self, sampler: *mut llama_sampler) {
        llama_sampler_chain_add(self.0, sampler);
    }

    pub fn sample(&self, context: &LlamaContext, idx: i32) -> i32 {
        unsafe { llama_sampler_sample(self.0, context.as_ptr(), idx) }
    }
}

impl Drop for LlamaSampler {
    fn drop(&mut self) {
        if !self.0.is_null() {
            unsafe {
                llama_sampler_free(self.0);
            }
        }
    }
}

unsafe impl Send for LlamaSampler {}
unsafe impl Sync for LlamaSampler {}
