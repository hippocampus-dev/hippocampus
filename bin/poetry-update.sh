#!/usr/bin/env bash

set -eo pipefail

args=()
flags=()
while (( "$#" )); do
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
      echo "Unsupported flag $1" >&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

function usage() {
  cat <<EOS
Usage:
   poetry-update.sh [--no-update]
EOS
}

pids=()

find . -type f -name 'pyproject.toml' | while IFS= read -r pyproject; do
  (
    cd "$(dirname "$pyproject")"
    poetry lock $flags
  ) &
  pids+=($!)
done

for pid in "${pids[@]}"; do
  wait $pid
done
