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
    generate!("llama_model_params_with_n_gpu_layers_and_n_cpu_moe")
    generate!("llama_model_load_from_file")
    generate!("llama_model_free")
    generate!("llama_model_get_vocab")
    generate!("llama_model_n_ctx_train")

    // Context functions
    generate!("llama_context_default_params")
    generate!("llama_context_params_with_n_ctx")
    generate!("llama_context_params_with_serving")
    generate!("llama_context_params_with_embeddings")
    generate!("llama_init_from_model")
    generate!("llama_free")
    generate!("llama_model_n_embd")
    generate!("llama_model_n_embd_out")

    // Vocab and tokenization
    generate!("llama_tokenize")
    generate!("llama_vocab_is_eog")
    generate!("llama_vocab_bos")
    generate!("llama_vocab_n_tokens")
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
    generate!("llama_sampler_init_greedy")
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
    pub fn llama_encode(ctx: *mut llama_context, batch: llama_batch) -> i32;
    pub fn llama_synchronize(ctx: *mut llama_context);
    pub fn llama_get_embeddings(ctx: *mut llama_context) -> *mut f32;
    pub fn llama_get_embeddings_ith(ctx: *mut llama_context, i: i32) -> *mut f32;
    pub fn llama_log_set(log_callback: GgmlLogCallback, user_data: *mut std::os::raw::c_void);
    pub fn llama_get_memory(ctx: *mut llama_context) -> *mut std::os::raw::c_void;
    pub fn llama_memory_seq_rm(
        mem: *mut std::os::raw::c_void,
        seq_id: i32,
        p0: i32,
        p1: i32,
    ) -> bool;
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
        n_cpu_moe: Option<i32>,
        use_mlock: bool,
    ) -> Result<Self, String> {
        ensure_backend_init();

        #[cfg(feature = "cuda")]
        const DEFAULT_GPU_LAYERS: i32 = 999;
        #[cfg(not(feature = "cuda"))]
        const DEFAULT_GPU_LAYERS: i32 = 0;

        let n_gpu_layers = n_gpu_layers.unwrap_or(DEFAULT_GPU_LAYERS);
        let n_cpu_moe = n_cpu_moe.unwrap_or(0);

        let path_c = std::ffi::CString::new(path.to_str().ok_or("Invalid path")?)
            .map_err(|e| format!("Failed to create C string: {e}"))?;

        unsafe {
            let model = if n_cpu_moe > 0 {
                let params = llama_model_params_with_n_gpu_layers_and_n_cpu_moe(
                    n_gpu_layers,
                    n_cpu_moe,
                    use_mlock,
                );
                llama_model_load_from_file(
                    path_c.as_ptr(),
                    autocxx::WithinUniquePtr::within_unique_ptr(params),
                )
            } else {
                let params = llama_model_params_with_n_gpu_layers(n_gpu_layers, use_mlock);
                llama_model_load_from_file(
                    path_c.as_ptr(),
                    autocxx::WithinUniquePtr::within_unique_ptr(params),
                )
            };

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

    /// Vocabulary size. Speculative decoding requires the draft and target models
    /// to share a vocabulary, so this is used to reject an incompatible pair.
    pub fn n_vocab(&self) -> i32 {
        unsafe { llama_vocab_n_tokens(llama_model_get_vocab(self.0)) }
    }

    pub fn n_embd(&self) -> i32 {
        unsafe { llama_model_n_embd(self.0) }
    }

    /// Output-embedding width; llama.cpp sizes and strides the embeddings
    /// buffer with this, which can be narrower than `n_embd` when a model
    /// sets `embedding_length_out`. Callers of `LlamaContext::embeddings`
    /// must use this as the row width.
    pub fn n_embd_out(&self) -> i32 {
        unsafe { llama_model_n_embd_out(self.0) }
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

pub struct LlamaContext {
    context: *mut llama_context,
    /// Row width of this context's embeddings buffer, which is `n_embd_out` and
    /// not always `n_embd`. `None` for a context created without embeddings,
    /// where there is no buffer to read at all.
    embeddings_row_width: Option<usize>,
    /// Rows the last `decode`/`encode` produced, and so the only rows holding
    /// embeddings for the current batch.
    ///
    /// This is what `embeddings` checks against, and it has to be tracked
    /// rather than assumed. llama.cpp reserves the output buffer for
    /// `n_seq_max` rows at construction and only ever grows it, to the largest
    /// `n_outputs` any one call has asked for (`output_reserve`,
    /// `llama-context.cpp`) -- so neither `n_batch` nor `n_ctx` says anything
    /// about what is live. Reading past this returns whatever the last shorter
    /// batch left behind, which is stale data wearing the shape of an answer.
    ///
    /// `Cell` rather than an atomic because a context is driven by one thread
    /// at a time; it is `Send` but deliberately not `Sync`, so there is nothing
    /// to synchronize with.
    last_outputs: std::cell::Cell<usize>,
}

/// Positions a batch asks for output on, which is what llama.cpp counts as
/// `n_outputs_all`. A null `logits` means only the final position, as
/// `llama_decode` reads it.
fn batch_output_count(batch: &llama_batch) -> usize {
    if batch.logits.is_null() {
        return usize::from(batch.n_tokens > 0);
    }
    (0..batch.n_tokens as usize)
        .filter(|&index| unsafe { *batch.logits.add(index) } != 0)
        .count()
}

impl LlamaContext {
    pub fn new(
        model: &LlamaModel,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
        n_seq_max: i32,
        n_threads: i32,
        n_threads_batch: i32,
        type_k: i32,
        type_v: i32,
        offload_kqv: bool,
    ) -> Result<Self, String> {
        let params = llama_context_params_with_serving(
            n_ctx,
            n_batch,
            n_ubatch,
            n_seq_max,
            n_threads,
            n_threads_batch,
            type_k,
            type_v,
            offload_kqv,
        );
        let context = unsafe {
            llama_init_from_model(
                model.as_ptr(),
                autocxx::WithinUniquePtr::within_unique_ptr(params),
            )
        };

        if context.is_null() {
            return Err("Failed to create context".to_string());
        }

        Ok(Self {
            context,
            embeddings_row_width: None,
            last_outputs: std::cell::Cell::new(0),
        })
    }

    /// Context that extracts per-token embeddings (post final norm) instead of
    /// serving generations: single sequence, `embeddings = true`, unpooled
    /// (`pooling_type = NONE`), default KV precision. Used by the audio
    /// pipeline for hidden-state extraction and the WavTokenizer decoder.
    pub fn new_with_embeddings(
        model: &LlamaModel,
        n_ctx: i32,
        n_batch: i32,
        n_ubatch: i32,
        n_threads: i32,
        n_threads_batch: i32,
    ) -> Result<Self, String> {
        let params = llama_context_params_with_embeddings(
            n_ctx,
            n_batch,
            n_ubatch,
            n_threads,
            n_threads_batch,
        );
        let context = unsafe {
            llama_init_from_model(
                model.as_ptr(),
                autocxx::WithinUniquePtr::within_unique_ptr(params),
            )
        };

        if context.is_null() {
            return Err("Failed to create embeddings context".to_string());
        }

        Ok(Self {
            context,
            embeddings_row_width: Some(model.n_embd_out().max(0) as usize),
            last_outputs: std::cell::Cell::new(0),
        })
    }

    pub fn as_ptr(&self) -> *mut llama_context {
        self.context
    }

    pub fn decode(&self, batch: llama_batch) -> Result<(), String> {
        let outputs = batch_output_count(&batch);
        let result = unsafe { llama_decode(self.context, batch) };
        if result != 0 {
            return Err(format!("Decode failed with code: {result}"));
        }
        self.last_outputs.set(outputs);
        Ok(())
    }

    /// Forward pass for encoder-style models (e.g. the `wavtokenizer-dec`
    /// vocoder architecture); causal models keep using `decode`.
    pub fn encode(&self, batch: llama_batch) -> Result<(), String> {
        let outputs = batch_output_count(&batch);
        let result = unsafe { llama_encode(self.context, batch) };
        if result != 0 {
            return Err(format!("Encode failed with code: {result}"));
        }
        self.last_outputs.set(outputs);
        Ok(())
    }

    pub fn synchronize(&self) {
        unsafe { llama_synchronize(self.context) };
    }

    /// Embeddings of all batch outputs, in output order (`n_outputs * n_embd`
    /// floats). Only valid after `decode`/`encode` on a context created with
    /// `new_with_embeddings` where every batch token requested output; the
    /// arguments are checked against the buffer the context actually reserved,
    /// so a wrong one is an error rather than a read past its end.
    pub fn embeddings(&self, n_outputs: usize, n_embd: usize) -> Result<Vec<f32>, String> {
        let Some(row_width) = self.embeddings_row_width else {
            return Err("This context was not created to extract embeddings".to_string());
        };
        if n_embd != row_width {
            return Err(format!(
                "Asked for {n_embd}-wide embeddings from a context whose rows are {row_width} wide"
            ));
        }
        let live = self.last_outputs.get();
        if n_outputs > live {
            return Err(format!(
                "Asked for {n_outputs} embedding rows but the last decode produced {live}"
            ));
        }

        let pointer = unsafe { llama_get_embeddings(self.context) };
        if pointer.is_null() {
            return Err("No embeddings available in this context".to_string());
        }
        let mut output = vec![0.0f32; n_outputs * n_embd];
        unsafe {
            std::ptr::copy_nonoverlapping(pointer, output.as_mut_ptr(), output.len());
        }
        Ok(output)
    }

    pub fn clear_sequence(&self, seq_id: i32) {
        unsafe {
            let memory = llama_get_memory(self.context);
            llama_memory_seq_rm(memory, seq_id, -1, -1);
        }
    }

    /// Remove the KV cells of `seq_id` from position `from_pos` (inclusive) to
    /// the end, keeping the `[0, from_pos)` prefix. Used by prompt-cache reuse to
    /// drop only the tail that diverges from a new prompt sharing a prefix.
    pub fn remove_sequence_from(&self, seq_id: i32, from_pos: i32) {
        unsafe {
            let memory = llama_get_memory(self.context);
            llama_memory_seq_rm(memory, seq_id, from_pos, -1);
        }
    }
}

impl Drop for LlamaContext {
    fn drop(&mut self) {
        if !self.context.is_null() {
            unsafe {
                llama_free(self.context);
            }
        }
    }
}

// Send, so a context can be created on one thread and driven from another (the
// processing loop owns one; the audio pipeline moves into spawn_blocking).
unsafe impl Send for LlamaContext {}
// Deliberately not Sync. llama.cpp allows one thread per context, and `decode`
// takes `&self` -- so a Sync context would let two threads decode the same one
// through shared references and race inside llama.cpp with nothing in the type
// to forbid it. Without Sync a `&LlamaContext` cannot cross a thread boundary
// at all, which is the invariant llama.cpp actually wants. Every caller already
// goes through `&mut self` or a mutex, so this costs nothing today; it stops
// the next caller from paying for it.

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

    /// A greedy (argmax) sampler chain. Draft-model speculative decoding proposes
    /// candidates greedily, so acceptance-by-equality against the target leaves
    /// its output distribution unchanged.
    pub fn new_greedy() -> Result<Self, String> {
        let chain = Self::new()?;
        let greedy = llama_sampler_init_greedy();
        if greedy.is_null() {
            return Err("Failed to create greedy sampler".to_string());
        }
        unsafe {
            chain.chain_add(greedy);
        }
        Ok(chain)
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
