# syntax=docker/dockerfile:1.4

FROM akamai/jsonnet:v2.33.3
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

ADD --chown=1001:1001 --chmod=700 https://github.com/jsonnet-bundler/jsonnet-bundler/releases/download/v0.5.1/jb-linux-amd64 /usr/local/bin/jb

USER root
RUN mkdir -p /var/jsonnet-bundler && chown 1001:1001 /var/jsonnet-bundler
RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    apk update && \
    apk upgrade && \
    apk add bash git ca-certificates
USER 1001

COPY --chown=1001:1001 --chmod=700 entrypoint.sh /bin/entrypoint.sh

WORKDIR /var/jsonnet-builder

ENTRYPOINT ["/bin/entrypoint.sh"]
