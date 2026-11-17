#!/usr/bin/env bash

# Marker file path
readonly INIT_MARKER="/var/run/container_initialized"
readonly DOWNLOAD_MODELS="${DOWNLOAD_MODELS:-all_bf16}"  # Default if not set, all, all_int8, all_bf16, all_nf4, or comma-separated list of models
readonly REPO_DIR="${REPO_DIR:-/workspace/YuE-exllamav2-UI}"
readonly PROJECT_DIR="${PROJECT_DIR:-/workspace/YuE-exllamav2-UI/src/yue}"
readonly MODEL_DIR="${MODEL_DIR:-/workspace/models}"

echo "DOWNLOAD_MODELS is: $DOWNLOAD_MODELS"

source /opt/conda/etc/profile.d/conda.sh
conda activate pyenv

# Define models with source (Hugging Face repo) and destination directory
declare -A MODELS_BF16
MODELS_BF16["YuE-s1-7B-anneal-en-cot"]="m-a-p/YuE-s1-7B-anneal-en-cot:${MODEL_DIR}/YuE-s1-7B-anneal-en-cot"
MODELS_BF16["YuE-s1-7B-anneal-en-icl"]="m-a-p/YuE-s1-7B-anneal-en-icl:${MODEL_DIR}/YuE-s1-7B-anneal-en-icl"
MODELS_BF16["YuE-s1-7B-anneal-jp-kr-cot"]="m-a-p/YuE-s1-7B-anneal-jp-kr-cot:${MODEL_DIR}/YuE-s1-7B-anneal-jp-kr-cot"
MODELS_BF16["YuE-s1-7B-anneal-jp-kr-icl"]="m-a-p/YuE-s1-7B-anneal-jp-kr-icl:${MODEL_DIR}/YuE-s1-7B-anneal-jp-kr-icl"
MODELS_BF16["YuE-s1-7B-anneal-zh-cot"]="m-a-p/YuE-s1-7B-anneal-zh-cot:${MODEL_DIR}/YuE-s1-7B-anneal-zh-cot"
MODELS_BF16["YuE-s1-7B-anneal-zh-icl"]="m-a-p/YuE-s1-7B-anneal-zh-icl:${MODEL_DIR}/YuE-s1-7B-anneal-zh-icl"
MODELS_BF16["YuE-s2-1B-general"]="m-a-p/YuE-s2-1B-general:${MODEL_DIR}/YuE-s2-1B-general"
MODELS_BF16["YuE-upsampler"]="m-a-p/YuE-upsampler:${MODEL_DIR}/YuE-upsampler"


declare -A MODELS_EXL2
MODELS_EXL2["YuE-s1-7B-anneal-en-cot-exl2-3.0bpw"]="Alissonerdx/YuE-s1-7B-anneal-en-cot-exl2-3.0bpw:${MODEL_DIR}/YuE-s1-7B-anneal-en-cot-exl2-3.0bpw"
MODELS_EXL2["YuE-s1-7B-anneal-en-cot-exl2-4.0bpw"]="Alissonerdx/YuE-s1-7B-anneal-en-cot-exl2-4.0bpw:${MODEL_DIR}/YuE-s1-7B-anneal-en-cot-exl2-4.0bpw"
MODELS_EXL2["YuE-s1-7B-anneal-en-cot-exl2-5.0bpw"]="Alissonerdx/YuE-s1-7B-anneal-en-cot-exl2-5.0bpw:${MODEL_DIR}/YuE-s1-7B-anneal-en-cot-exl2-5.0bpw"
MODELS_EXL2["YuE-s1-7B-anneal-jp-kr-cot-5.0bpw"]="cpchan/YuE-s1-7B-anneal-jp-kr-cot-5.0bpw:${MODEL_DIR}/YuE-s1-7B-anneal-jp-kr-cot-5.0bpw"
MODELS_EXL2["YuE-s1-7B-anneal-jp-kr-icl-5.0bpw"]="cpchan/YuE-s1-7B-anneal-jp-kr-icl-5.0bpw:${MODEL_DIR}/YuE-s1-7B-anneal-jp-kr-icl-5.0bpw"
MODELS_EXL2["YuE-s1-7B-anneal-en-cot-exl2-6.0bpw"]="Alissonerdx/YuE-s1-7B-anneal-en-cot-exl2-6.0bpw:${MODEL_DIR}/YuE-s1-7B-anneal-en-cot-exl2-6.0bpw"
MODELS_EXL2["YuE-s1-7B-anneal-en-cot-exl2-8.0bpw"]="Alissonerdx/YuE-s1-7B-anneal-en-cot-exl2-8.0bpw:${MODEL_DIR}/YuE-s1-7B-anneal-en-cot-exl2-8.0bpw"
MODELS_EXL2["YuE-s2-1B-general-exl2-3.0bpw"]="Alissonerdx/YuE-s2-1B-general-exl2-3.0bpw:${MODEL_DIR}/YuE-s2-1B-general-exl2-3.0bpw"
MODELS_EXL2["YuE-s2-1B-general-exl2-4.0bpw"]="Alissonerdx/YuE-s2-1B-general-exl2-4.0bpw:${MODEL_DIR}/YuE-s2-1B-general-exl2-4.0bpw"
MODELS_EXL2["YuE-s2-1B-general-exl2-5.0bpw"]="Alissonerdx/YuE-s2-1B-general-exl2-5.0bpw:${MODEL_DIR}/YuE-s2-1B-general-exl2-5.0bpw"
MODELS_EXL2["YuE-s2-1B-general-exl2-6.0bpw"]="Alissonerdx/YuE-s2-1B-general-exl2-6.0bpw:${MODEL_DIR}/YuE-s2-1B-general-exl2-6.0bpw"
MODELS_EXL2["YuE-s2-1B-general-exl2-8.0bpw"]="Alissonerdx/YuE-s2-1B-general-exl2-8.0bpw:${MODEL_DIR}/YuE-s2-1B-general-exl2-8.0bpw"
MODELS_EXL2["YuE-upsampler"]="m-a-p/YuE-upsampler:${MODEL_DIR}/YuE-upsampler"

if [ ! -f "$INIT_MARKER" ]; then
  echo "First-time initialization..."

  echo "Installing CUDA nvcc..."
  conda install -y -c nvidia cuda-nvcc --override-channels

  # echo "Installing exllamav2"
  # git clone https://github.com/turboderp/exllamav2
  # cd exllamav2
  # pip install -r requirements.txt
  # pip install .

  # cd ..

  echo "Installing dependencies from requirements.txt..."
  pip install --no-cache-dir -r "${REPO_DIR}/requirements.txt"


  #cp -r $REPO_DIR/transformers/ /opt/conda/envs/pyenv/lib/python3.12/site-packages

  if [ ! -d "${PROJECT_DIR}/xcodec_mini_infer" ]; then
    huggingface-cli download m-a-p/xcodec_mini_infer --local-dir "${PROJECT_DIR}/xcodec_mini_infer"
  else
    echo "Skipping the model xcodec_mini_infer download because it already exists."
  fi

  if [ "$DOWNLOAD_MODELS" != "false" ]; then
    echo "Downloading selected models..."

    if [ "$DOWNLOAD_MODELS" = "all_bf16" ]; then
      selected_models=("${!MODELS_BF16[@]}")
      declare -A models
      for key in "${!MODELS_BF16[@]}"; do
        models[$key]="${MODELS_BF16[$key]}"
      done
    elif [ "$DOWNLOAD_MODELS" = "all_exl2" ]; then
      selected_models=("${!MODELS_EXL2[@]}")
      declare -A models
      for key in "${!MODELS_EXL2[@]}"; do
        models[$key]="${MODELS_EXL2[$key]}"
      done
    elif [ "$DOWNLOAD_MODELS" = "all" ]; then
      selected_models=("${!MODELS_BF16[@]}" "${!MODELS_EXL2[@]}")
      declare -A models
      for key in "${!MODELS_BF16[@]}"; do
        models[$key]="${MODELS_BF16[$key]}"
      done
      for key in "${!MODELS_EXL2[@]}"; do
        models[$key]="${MODELS_EXL2[$key]}"
      done
    else
      IFS=, read -r -a selected_models <<< "$DOWNLOAD_MODELS"
      declare -A models
      for model in "${selected_models[@]}"; do
        if [ -v "MODELS_BF16[$model]" ]; then
          models[$model]="${MODELS_BF16[$model]}"
        elif [ -v "MODELS_EXL2[$model]" ]; then
          models[$model]="${MODELS_EXL2[$model]}"
        else
          echo "Warning: Model $model is not recognized. Skipping." 1>&2
        fi
      done
    fi
    for model in "${selected_models[@]}"; do
      IFS=: read -r source destination <<< "${models[$model]}"

      if [ ! -d "$destination" ]; then
        echo "Downloading model: $model from $source to $destination"
        huggingface-cli download "$source" --local-dir "$destination"
      else
        echo "Skipping the model $model because it already exists in ${destination}."
      fi
    done
  else
    echo "DOWNLOAD_MODELS is false, skipping model downloads."
  fi

  # Create marker file
  touch "$INIT_MARKER"
  echo "Initialization complete."
else
  echo "Container already initialized. Skipping first-time setup."
fi

echo "Adding environment variables"
export PATH="${REPO_DIR}:${PROJECT_DIR}:${PROJECT_DIR}/xcodec_mini_infer:${PROJECT_DIR}/xcodec_mini_infer:${PROJECT_DIR}/xcodec_mini_infer/descriptaudiocodec:${PATH}"

echo "$PATH"
echo "$PYTHONPATH"

cd "$PROJECT_DIR"

# Use conda python instead of system python
echo "Starting Gradio interface..."
python interface.py &

# Use debugpy for debugging
# exec python -m debugpy --wait-for-client --listen 0.0.0.0:5678 gradio_interface.py

# echo "Starting Tensorboard interface..."
# $CONDA_DIR/bin/conda run -n pyenv tensorboard --logdir_spec=/workspace/outputs --bind_all --port 6006 &
wait
