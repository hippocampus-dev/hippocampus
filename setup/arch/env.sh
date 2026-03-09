#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

_GROUPS=(
  docker
  libvirt
  wireshark
)

_SERVICES=(
  libvirtd.service
  docker.service
  minikube.service
  minikube-context.service
  minikube-reset.service
  minikube-socat.service
  hippocampus-compose.service
  bluetooth.service
  lifecycle.service
  nfs-server.service
  xrdp.service
  backup.timer
  sync.timer
  claude-hourly-task.timer
)

_USER_SERVICES=(
  pactl-subscribe.service
)
