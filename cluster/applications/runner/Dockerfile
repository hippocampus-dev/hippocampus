# syntax=docker/dockerfile:1.4

FROM ubuntu:24.04
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

ARG DEBIAN_FRONTEND=noninteractive

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl jq ruby ca-certificates gpg gpg-agent netcat-traditional libicu74

ENV GITHUB_CLI_VERSION=2.71.2
ENV KUSTOMIZE_VERSION=5.8.0

RUN curl -fsSL https://github.com/cli/cli/releases/download/v${GITHUB_CLI_VERSION}/gh_${GITHUB_CLI_VERSION}_linux_amd64.tar.gz | tar zx --no-same-owner -C /usr/local/bin --strip-components=2 gh_${GITHUB_CLI_VERSION}_linux_amd64/bin/gh && \
    curl -fsSL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | tar zx --no-same-owner -C /usr/local/bin

ADD --chmod=755 https://dl.k8s.io/release/v1.34.0/bin/linux/amd64/kubectl /usr/local/bin/kubectl
