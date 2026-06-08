#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

find /var/log/pods -type f -name "*.log.*" -delete
