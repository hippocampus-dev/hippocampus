#!/usr/bin/env bash

set -e

function usage() {
  cat <<EOS
Usage:
   uv-update.sh [--frozen]
EOS
}

args=()
flags=()
while (( $# )); do
  case "${1}" in
    -h|--help)
      usage
      exit 0
      ;;
    --frozen)
      flags+=("${1}")
      shift
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag ${1}" 1>&2
      exit 1
      ;;
    *)
      args+=("${1}")
      shift
      ;;
  esac
done

pids=()

find . -type f -name uv.lock | while IFS= read -r uvproject; do
  (
    cd $(dirname ${uvproject})
    uv lock ${flags}
  ) &
  pids+=($!)
done

for pid in "${pids[@]}"; do
  wait ${pid}
done
