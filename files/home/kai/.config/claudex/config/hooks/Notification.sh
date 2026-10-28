#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

message=$(echo "$json" | jq -r '.message')
title=$(armyknife mcp call http://127.0.0.1:47100/sse get_pane_title "{\"pane\":\"$TMUX_PANE\"}" 2>/dev/null || true)

pane_url=""
if [ -n "$TMUX_PANE" ]; then
  # RFC 3986 reads % as the start of a percent-encoded octet, so the pane id travels without its leading %
  pane_url="file:///tmux/pane/${TMUX_PANE#%}"$'\n\n'
fi

armyknife mcp call http://127.0.0.1:47101/sse notify "$(jq -nc --arg summary "${title:-Agent}" --arg body "${pane_url}${message}" '{summary:$summary,body:$body,urgency:"low","expire-time":10}')" > /dev/null 2>&1 || true
