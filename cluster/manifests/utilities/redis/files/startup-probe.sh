#!/usr/bin/env bash

set -eo pipefail

exec > /dev/null 2>&1

# Avoid LOADING rejection
# https://github.com/redis/redis/blob/5e3be1be09c947810732e7be2a4bb1b0ed75de4a/src/server.c#L4050-L4061
if [ "$(redis-cli ping)" != "PONG" ]; then
  exit 1
fi
