#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

_GROUPS=(
  docker
  libvirt
  video
  wireshark
)

_SERVICES=(
  libvirtd.service
  docker.service
  docker-rootless.service
  minikube.service
  minikube-context.service
  minikube-reset.service
  minikube-socat.service
  hippocampus-compose.service
  bluetooth.service
  lifecycle.service
  nfs-server.service
  xrdp.service
  microsocks.service
  run-claudex-pts.mount
  cpu-frequency-limit.service
  backup.timer
  mirror-registry-garbage-collect.timer
  scrub.timer
  sync.timer
)

_USER_SERVICES=(
  pactl-subscribe.service
  ttyd.service
  square-webcam.service
  claude-daily-task.timer
)
