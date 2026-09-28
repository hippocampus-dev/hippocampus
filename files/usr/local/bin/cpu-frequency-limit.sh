#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

MAXIMUM_KILOHERTZ=5000000

for policy in /sys/devices/system/cpu/cpu*/cpufreq/scaling_max_freq; do
  echo "$MAXIMUM_KILOHERTZ" > "$policy"
done
