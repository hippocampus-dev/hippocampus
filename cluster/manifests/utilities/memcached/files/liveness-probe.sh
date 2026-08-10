#!/usr/bin/env bash

set -e

exec > /dev/null 2>&1

# The image ships no memcached client, so the stats port is read through bash's /dev/tcp
exec 3<>/dev/tcp/127.0.0.1/5000
printf "stats all\r\n" >&3

downed=
while IFS=$' \t\r' read -r -t 1 stat name value <&3; do
  if [ "$stat" = "STAT" ] && [ "$name" = "num_servers_down" ]; then
    downed="$value"
    break
  fi
done
exec 3<&-

# Compared as a string so that a missing or malformed value fails instead of erroring past the check
if [ "$downed" != "0" ]; then
  exit 1
fi
