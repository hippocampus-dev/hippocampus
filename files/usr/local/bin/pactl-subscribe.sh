#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

pactl subscribe | while IFS= read -r event; do
  if echo "$event" | grep -q "Event 'new' on sink #"; then
    pactl set-sink-volume @DEFAULT_SINK@ 25%
  fi
done
