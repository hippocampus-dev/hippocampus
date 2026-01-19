#!/usr/bin/env bash

set -eo pipefail

function usage() {
  cat <<EOS
Usage:
   cargo-update.sh
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

while IFS= read -r cargo; do
  (
    cd "$(dirname "$cargo")"
    cargo update ${flags}
  ) &
  pids+=($!)
done < <(find . -type f -name Cargo.toml)

for pid in "${pids[@]}"; do
  wait "$pid"
done
