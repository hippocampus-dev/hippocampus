#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

_GROUPS=(
  docker
)

_SERVICES=(
  sync.timer
)

_USER_SERVICES=(
)
