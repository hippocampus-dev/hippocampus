#!/usr/bin/env bash

set -eo pipefail

function usage() {
  cat <<EOS
Usage:
   poetry-update.sh [--no-update]
EOS
}

args=()
flags=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --no-update)
      flags+=("$1")
      shift
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

pids=()

while IFS= read -r poetryproject; do
  (
    cd "$(dirname "$poetryproject")"
    uvx poetry lock ${flags}
  ) &
  pids+=($!)
done < <(find . -type f -name poetry.lock)

for pid in "${pids[@]}"; do
  wait "$pid"
done
