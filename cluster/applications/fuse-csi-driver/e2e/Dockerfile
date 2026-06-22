# syntax=docker/dockerfile:1.4

FROM golang:1.25-bookworm AS builder

ENV CGO_ENABLED=0

WORKDIR /opt/builder

COPY go.mod go.sum /opt/builder/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY bin/fuser /opt/builder/bin/fuser
COPY bin/fuse-placeholder /opt/builder/bin/fuse-placeholder
COPY internal /opt/builder/internal

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -o /usr/local/bin/fuser -ldflags="-s -w" /opt/builder/bin/fuser/*.go && \
    go build -trimpath -o /usr/local/bin/fuse-placeholder -ldflags="-s -w" /opt/builder/bin/fuse-placeholder/*.go

FROM debian:bookworm-slim
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd && \
    echo "nonroot:x:65532:" >> /etc/group

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends unzip

ARG RCLONE_VERSION=1.69.3
ADD --chmod=644 https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.zip /tmp/rclone.zip
RUN unzip -j /tmp/rclone.zip '*/rclone' -d /tmp && install -m 755 /tmp/rclone /usr/local/bin/rclone && rm /tmp/rclone.zip /tmp/rclone

COPY --link --from=builder /usr/local/bin/fuser /usr/local/bin/fuser
COPY --link --from=builder /usr/local/bin/fuse-placeholder /usr/local/bin/fuse-placeholder

USER 65532

ENTRYPOINT ["/usr/local/bin/fuser"]
