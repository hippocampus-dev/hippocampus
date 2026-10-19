#!/usr/bin/env bash
set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

STORAGE_IMAGE=/var/lib/docker.img
STORAGE_SIZE=20G
LOOP_DEVICE=/dev/loop0
LOG_FILE=/var/log/dockerd.log

# The container's /dev carries no loop device node
sudo mknod "${LOOP_DEVICE}" b 7 0

# https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-run-docker-with-kata.md
sudo truncate -s "${STORAGE_SIZE}" "${STORAGE_IMAGE}"
sudo mkfs.ext4 -q -F "${STORAGE_IMAGE}"
sudo mkdir -p /var/lib/docker
sudo losetup "${LOOP_DEVICE}" "${STORAGE_IMAGE}"
sudo mount "${LOOP_DEVICE}" /var/lib/docker

sudo install -m 644 -o "$(id -u)" /dev/null "${LOG_FILE}"
sudo dockerd --group "$(id -gn)" </dev/null >"${LOG_FILE}" 2>&1 &

for i in $(seq 1 60); do
  if docker version >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done

cat "${LOG_FILE}" >&2
exit 1
