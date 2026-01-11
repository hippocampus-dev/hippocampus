# syntax=docker/dockerfile:1.4

FROM golang:1.24-bullseye AS builder

ENV CGO_ENABLED=0

WORKDIR /opt/builder

COPY go.mod go.sum /opt/builder/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY main.go /opt/builder/main.go
COPY api /opt/builder/api
COPY internal /opt/builder/internal

ARG LD_FLAGS="-s -w"
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/main -ldflags="${LD_FLAGS}" /opt/builder/*.go

FROM debian:bookworm-slim AS jsonnet-bundler

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends git ca-certificates

ADD --chmod=700 https://github.com/jsonnet-bundler/jsonnet-bundler/releases/download/v0.5.1/jb-linux-amd64 /usr/local/bin/jb

WORKDIR /opt/jsonnet
COPY jsonnetfile.json jsonnetfile.lock.json /opt/jsonnet/
RUN jb install
COPY common.libsonnet /opt/jsonnet/vendor/common.libsonnet

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/bin/main /usr/local/bin/grafana-manifest-controller
COPY --link --from=jsonnet-bundler /opt/jsonnet/vendor /opt/jsonnet/vendor

USER 65532

ENTRYPOINT ["/usr/local/bin/grafana-manifest-controller"]
