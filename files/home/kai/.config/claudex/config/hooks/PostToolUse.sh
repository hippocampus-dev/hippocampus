#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

file_path=$(echo "$json" | jq -r '.tool_input.file_path')

if [ -z "$file_path" ] || [ "$file_path" = "null" ] || [ ! -f "$file_path" ]; then
  exit 0
fi

make fmt 2>/dev/null || true

sed -i --follow-symlinks 's/[[:space:]]*$//' "$file_path"

if [ -n "$(tail -c1 "$file_path")" ]; then
  echo >> "$file_path"
fi
