#!/usr/bin/env bash

set -eo pipefail

exec > /dev/null 2>&1

downed=$(echo "stats all" | nc -q0 127.0.0.1 5000 | grep "STAT num_servers_down" | awk '{print $NF}')
if [ $downed -gt 0 ]; then
  exit 1
fi
