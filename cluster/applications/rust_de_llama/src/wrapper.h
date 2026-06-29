#pragma once

#include <string>
#include <vector>
#include <cstdio>

#include "llama.h"
#include "ggml-backend.h"
#include "ggml-cpu.h"
#include "ggml.h"

inline llama_model_params llama_model_params_with_n_gpu_layers(int32_t n_gpu_layers) {
    llama_model_params params = llama_model_default_params();
    params.n_gpu_layers = n_gpu_layers;
    return params;
}

inline llama_model_params llama_model_params_with_n_gpu_layers_and_n_cpu_moe(int32_t n_gpu_layers, int32_t n_cpu_moe) {
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
