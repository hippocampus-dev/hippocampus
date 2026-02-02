#!/usr/bin/env bash

set -eo pipefail

pactl subscribe | while IFS= read -r event; do
  if echo "$event" | grep -q "Event 'new' on sink #"; then
    pactl set-sink-volume @DEFAULT_SINK@ 25%
  fi
done
