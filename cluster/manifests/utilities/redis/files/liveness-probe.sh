#!/usr/bin/env bash

set -e

exec > /dev/null 2>&1

# Avoid LOADING rejection
# https://github.com/redis/redis/blob/5e3be1be09c947810732e7be2a4bb1b0ed75de4a/src/server.c#L4050-L4061
if [ "$(redis-cli ping)" != "PONG" ]; then
  exit 1
fi

if redis-cli info | grep role:master; then
  if ! redis-cli info | grep -E "connected_slaves:[$((QUORUM - 1))-9]([0-9]*)?"; then
    exit 1
  fi
else
  if ! redis-cli info | grep master_link_status:up && redis-cli info | grep master_sync_in_progress:0 && redis-cli -p 26379 info | grep -v sentinel_masters:0; then
    exit 1
  fi
fi
