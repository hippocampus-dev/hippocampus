#!/usr/bin/env bash

set -e

if [ ! -f /tmp/uptime ]; then
  date +%s > /tmp/uptime
fi

now=$(date +%s)
uptime=$(cat /tmp/uptime)
entropy=$(( RANDOM % 30 * 600 )) # 5 hours
# Restart daily(19-24 hours) to reset fluentd-prometheus counter
if [ $((now - uptime)) -gt $((86400 + entropy)) ]; then
  rm /tmp/uptime
  exit 1
fi
