#!/usr/bin/env bash

set -e

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
)

_USER_SERVICES=(
  pactl-subscribe.service
)
