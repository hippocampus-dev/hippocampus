#!/usr/bin/env bash

set -e

function usage() {
  cat <<EOS
Usage:
   gomod-update.sh
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

find . -type f -name go.mod | while IFS= read -r gomod; do
  (
    cd $(dirname ${gomod})
    go mod tidy ${flags}
  ) &
  pids+=($!)
done

for pid in "${pids[@]}"; do
  wait ${pid}
done
