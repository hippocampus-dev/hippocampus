#pragma once

#include "llama.h"
#include "ggml-backend.h"
#include "ggml-cpu.h"
#include "ggml.h"

inline llama_model_params llama_model_params_with_n_gpu_layers(int32_t n_gpu_layers) {
    llama_model_params params = llama_model_default_params();
    params.n_gpu_layers = n_gpu_layers;
    return params;
}

inline llama_context_params llama_context_params_with_n_ctx(int32_t n_ctx) {
    llama_context_params params = llama_context_default_params();
    params.n_ctx = n_ctx;
    return params;
}

inline llama_context_params llama_context_params_with_n_ctx_and_n_ubatch(int32_t n_ctx, int32_t n_ubatch) {
    llama_context_params params = llama_context_default_params();
    params.n_ctx = n_ctx;
    params.n_ubatch = n_ubatch;
    return params;
}
