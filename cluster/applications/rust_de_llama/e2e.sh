#!/usr/bin/env bash
set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   e2e.sh

Renders speech through the audio pipeline's pretrained stages and asserts an
ASR model reads back the words that went in. No cluster is needed: the GGUFs
are fetched into models/ and the checks run as release tests in-process.

The projection is the pipeline's only trained stage and this repository ships
no weights for it, so /v1/audio/speech itself is out of scope here -- what is
verifiable without training is the talker, the decoder and the inverse STFT.
EOS
}

while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unsupported argument $1" 1>&2
      exit 1
      ;;
  esac
done

cd "$(dirname "${BASH_SOURCE[0]}")"

# Release, so a full-length synthesis does not take minutes of unoptimized
# iSTFT. Audio itself runs under any profile; see build.rs on NDEBUG.
readonly MODELS_DIR=models
readonly WAVTOKENIZER_URL=https://huggingface.co/ggml-org/WavTokenizer/resolve/main/WavTokenizer-Large-75-F16.gguf

function fetch_model() {
  local name="$1" url="$2"
  if [ -s "${MODELS_DIR}/${name}" ]; then
    return
  fi
  echo "fetching ${name}"
  # Downloaded aside and moved on success: curl leaves the partial body in
  # place once its retries are exhausted, and the guard above would then take
  # the truncated file for a complete one on the next run.
  curl -fsSL --retry 5 --retry-all-errors \
    -o "${MODELS_DIR}/${name}.partial" "${url}"
  mv "${MODELS_DIR}/${name}.partial" "${MODELS_DIR}/${name}"
}

fetch_model WavTokenizer-Large-75-F16.gguf "${WAVTOKENIZER_URL}"

if [ ! -s "${MODELS_DIR}/OuteTTS-0.2-500M-F16.gguf" ]; then
  echo "fetching and converting OuteTTS-0.2-500M"
  hf_dir="$(mktemp -d)"
  trap 'rm -rf "${hf_dir}"' EXIT
  for file in config.json generation_config.json tokenizer.json tokenizer_config.json \
              vocab.json merges.txt special_tokens_map.json added_tokens.json model.safetensors; do
    curl -fsSL --retry 5 --retry-all-errors -o "${hf_dir}/${file}" \
      "https://huggingface.co/OuteAI/OuteTTS-0.2-500M/resolve/main/${file}"
  done
  # The converter needs sentencepiece importable to reach its BPE fallback.
  uv run --no-project --python 3.12 \
    --with torch --with numpy --with safetensors --with transformers --with sentencepiece \
    --with-editable llama.cpp/gguf-py \
    python llama.cpp/convert_hf_to_gguf.py "${hf_dir}" \
      --outfile "${MODELS_DIR}/OuteTTS-0.2-500M-F16.gguf" --outtype f16
fi

# Each test renders a WAV named after itself into the temporary directory.
rm -f "${TMPDIR:-/tmp}"/rust_de_llama-golden-*.wav \
      "${TMPDIR:-/tmp}"/rust_de_llama-talker-native-*.wav
cargo test --release --bin server -- --nocapture \
  audio::wavtokenizer::tests::test_decode_golden_codes \
  audio::talker::tests::test_generate_native_speech

uv run --no-project --python 3.12 \
  --with librosa --with transformers --with torch \
  python whisper/transcribe.py \
    "${TMPDIR:-/tmp}"/rust_de_llama-golden-*.wav \
      "$(cat tests/fixtures/wavtokenizer_golden_text.txt)" \
    "${TMPDIR:-/tmp}"/rust_de_llama-talker-native-*.wav \
      "the quick brown fox jumps over the lazy dog"

echo "e2e: ok"
