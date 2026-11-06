#pragma once

#include <string>
#include <vector>
#include <cstdio>

#include "llama.h"
#include "ggml-backend.h"
#include "ggml-cpu.h"
#include "ggml.h"

inline llama_model_params llama_model_params_with_n_gpu_layers(int32_t n_gpu_layers, bool use_mlock) {
    llama_model_params params = llama_model_default_params();
    params.n_gpu_layers = n_gpu_layers;
    params.use_mlock = use_mlock;
    return params;
}

inline llama_model_params llama_model_params_with_n_gpu_layers_and_n_cpu_moe(int32_t n_gpu_layers, int32_t n_cpu_moe, bool use_mlock) {
    static std::vector<std::string> patterns;
    static std::vector<llama_model_tensor_buft_override> overrides;

    patterns.clear();
    overrides.clear();

    for (int32_t i = 0; i < n_cpu_moe; i++) {
        char buffer[128];
        std::snprintf(buffer, sizeof(buffer), "blk\\.%d\\.ffn_(up|down|gate)_(ch|)exps", i);
        patterns.emplace_back(buffer);
    }

    for (const auto & pattern : patterns) {
        overrides.push_back({pattern.c_str(), ggml_backend_cpu_buffer_type()});
    }
    overrides.push_back({nullptr, nullptr});

    llama_model_params params = llama_model_default_params();
    params.n_gpu_layers = n_gpu_layers;
    params.tensor_buft_overrides = overrides.data();
    params.use_mlock = use_mlock;
    return params;
}

inline llama_context_params llama_context_params_with_n_ctx(int32_t n_ctx) {
    llama_context_params params = llama_context_default_params();
    params.n_ctx = n_ctx;
    return params;
}

inline llama_context_params llama_context_params_with_serving(int32_t n_ctx, int32_t n_batch, int32_t n_ubatch, int32_t n_seq_max, int32_t n_threads, int32_t n_threads_batch, int32_t type_k, int32_t type_v, bool offload_kqv) {
    llama_context_params params = llama_context_default_params();
    params.n_ctx = n_ctx;
    params.n_batch = n_batch;
    params.n_ubatch = n_ubatch;
    params.n_seq_max = n_seq_max;
    params.n_threads = n_threads;
    params.n_threads_batch = n_threads_batch;
    params.type_k = static_cast<ggml_type>(type_k);
    params.type_v = static_cast<ggml_type>(type_v);
    params.offload_kqv = offload_kqv;
    // Pin the per-sequence KV layout the serving math assumes: with kv_unified =
    // true every sequence may address the whole cache instead of a fixed
    // n_ctx / n_seq_max slice, so one long request is not capped by the slot
    // count and idle slots' cells are not stranded. Sequences over-commit the
    // shared cells exactly as llama-server's slots do under its own default.
    // Set explicitly so a future default flip cannot silently mis-size the
    // context that admission control and the completion boundary rely on
    // (n_ctx_seq in parallel.rs).
    params.kv_unified = true;
    return params;
}

inline llama_context_params llama_context_params_with_embeddings(int32_t n_ctx, int32_t n_batch, int32_t n_ubatch, int32_t n_threads, int32_t n_threads_batch) {
    llama_context_params params = llama_context_default_params();
    params.n_ctx = n_ctx;
    params.n_batch = n_batch;
    params.n_ubatch = n_ubatch;
    params.n_seq_max = 1;
    params.n_threads = n_threads;
    params.n_threads_batch = n_threads_batch;
    params.embeddings = true;
    // Readers copy n_tokens * n_embd rows; a model-declared pooling type would
    // shrink the buffer under them.
    params.pooling_type = LLAMA_POOLING_TYPE_NONE;
    return params;
}
