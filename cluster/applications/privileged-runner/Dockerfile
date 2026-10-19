# syntax=docker/dockerfile:1.4

FROM ghcr.io/hippocampus-dev/hippocampus/runner:main
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

ARG DEBIAN_FRONTEND=noninteractive

# e2fsprogs formats the image a daemon mounts on its storage directory, since virtio-fs cannot be the upper layer of overlayfs: https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-run-docker-with-kata.md

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends iptables e2fsprogs

ENV DOCKER_VERSION=29.7.2
ENV BUILDX_VERSION=0.36.1

RUN curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz | tar zx --no-same-owner -C /usr/local/bin --strip-components=1

ADD --chmod=755 https://github.com/docker/buildx/releases/download/v${BUILDX_VERSION}/buildx-v${BUILDX_VERSION}.linux-amd64 /usr/local/lib/docker/cli-plugins/docker-buildx

COPY --chmod=755 start-dockerd.sh /usr/local/bin/start-dockerd.sh
