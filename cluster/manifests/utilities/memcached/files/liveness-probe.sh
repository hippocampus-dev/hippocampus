#!/usr/bin/env bash

set -e

exec > /dev/null 2>&1

# The image ships no memcached client
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

# Compared as a string: a missing or malformed value must fail
if [ "$downed" != "0" ]; then
  exit 1
fi
