# rust_de_llama

<!-- TOC -->
* [rust_de_llama](#rust_de_llama)
  * [Features](#features)
  * [Requirements](#requirements)
  * [Development](#development)
<!-- TOC -->

rust_de_llama is a Rust HTTP server that provides an OpenAI-compatible chat completion and speech synthesis API powered by llama.cpp.

## Features

- [x] `POST /v1/chat/completions`, which also speaks its reply when asked for `modalities: ["text", "audio"]`
- [x] `POST /v1/audio/speech`, backed by a Thinker-Talker pipeline declared as `[audio_pipelines]` in `models/models.toml`
- [x] Speech from the text itself, read by a pretrained talker and rendered by a WavTokenizer decoder
- [ ] Speech from the thinker's hidden states, bridged into the talker's embedding space by a projection trained per thinker with `notebooks/thinker-talker-projection.ipynb`

## Requirements

The CPU build targets the `x86-64-v3` ISA baseline (AVX2 + FMA + BMI2), standard on x86 CPUs since ~2015.
Override it for older hardware with the `RUST_DE_LLAMA_MARCH` environment variable at build time:

```sh
$ export RUST_DE_LLAMA_MARCH=x86-64-v2
```

The `cuda` feature, which `make dev` builds with, compiles llama.cpp through cmake instead and needs a CUDA toolkit, taking `nvcc` from `PATH`.
Point it at another one with the `CUDACXX` environment variable at build time:

```sh
$ export CUDACXX=/opt/cuda/bin/nvcc
```

## Development

```sh
$ make dev
```
