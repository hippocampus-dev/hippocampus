#!/usr/bin/env bash

set -e

function usage() {
  cat <<EOS
Usage:
   cargo-update.sh
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

find . -type f -name Cargo.toml | while IFS= read -r cargo; do
  (
    cd $(dirname ${cargo})
    cargo update ${flags}
  ) &
  pids+=($!)
done

for pid in "${pids[@]}"; do
  wait ${pid}
done
