#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

transcript_path=$(echo "$json" | jq -re '.transcript_path')
session_id=$(echo "$json" | jq -re '.session_id')

[ -f "$transcript_path" ] || exit 0

transcript=$(jq -re 'select(.type == "assistant" or .type == "human") | .message.content' "$transcript_path" 2>/dev/null | head -c 100000)

[ -n "$transcript" ] || exit 0

#claudex --print --dangerously-skip-permissions \
#  -p "Extract key decisions and their reasoning from the following session transcript, then store each as a separate memory using the graphiti add_memory tool. Focus on: architectural decisions, design choices, rejected alternatives and why, configuration choices. Skip trivial or routine actions. If there are no notable decisions, do nothing. Be concise.\n\nSession ${session_id} transcript:\n\n${transcript}" \
#  > /dev/null 2>&1 &
