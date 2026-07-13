# rust_de_llama

<!-- TOC -->
* [rust_de_llama](#rust_de_llama)
  * [Requirements](#requirements)
  * [Development](#development)
<!-- TOC -->

rust_de_llama is a Rust HTTP server that provides an OpenAI-compatible chat completion API powered by llama.cpp.

## Requirements

The release CPU build targets the `x86-64-v3` ISA baseline (AVX2 + FMA + BMI2), standard on x86 CPUs since ~2015. Override it for older hardware with the `RUST_DE_LLAMA_MARCH` environment variable at build time:

```sh
$ export RUST_DE_LLAMA_MARCH=x86-64-v2
```

## Development

```sh
$ make dev
```
