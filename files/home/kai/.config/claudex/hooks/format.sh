#!/usr/bin/env bash

set -e

json=$(cat - | jq .)

file_path=$(echo $json | jq -r .tool_input.file_path)

make fmt 2>/dev/null || true

sed -i 's/[[:space:]]*$//' $file_path

if [[ -n $(tail -c1 $file_path) ]]; then
  echo >> $file_path
fi
