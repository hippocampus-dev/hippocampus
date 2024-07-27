FROM ubuntu:22.04

ARG DEBIAN_FRONTEND=noninteractive

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl jq ca-certificates

ENV GITHUB_CLI_VERSION=2.53.0
RUN curl -fsSL https://github.com/cli/cli/releases/download/v${GITHUB_CLI_VERSION}/gh_${GITHUB_CLI_VERSION}_linux_amd64.tar.gz | tar zx --no-same-owner -C /usr/local/bin --strip-components=2 gh_${GITHUB_CLI_VERSION}_linux_amd64/bin/gh
