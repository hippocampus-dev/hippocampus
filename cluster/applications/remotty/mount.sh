#!/usr/bin/env bash
set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

while [ ! -S "${SOCKET_DIRECTORY}/fuse.sock" ]; do sleep 1; done
while ! nc -z 127.0.0.1 65533 2>/dev/null; do sleep 1; done
while ! nc -z 127.0.0.1 65534 2>/dev/null; do sleep 1; done

exec fuser "${SOCKET_DIRECTORY}" rclone mount2 \
  ':sftp,host=127.0.0.1,port=65533,user=nonroot,key_file=/dev/fd/3:' \
  '{}' --vfs-cache-mode off --allow-non-empty \
  3< <(curl -fsSL -u "$CHISEL_AUTH" http://127.0.0.1:65534/v1/key)
