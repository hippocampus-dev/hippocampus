# syntax=docker/dockerfile:1.4

FROM golang:1.27-bookworm AS builder

ENV CGO_ENABLED=0

WORKDIR /opt/builder

COPY go.mod /opt/builder/

COPY main.go /opt/builder/main.go
ARG LD_FLAGS="-s -w"
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/main -ldflags="${LD_FLAGS}" /opt/builder/*.go

FROM debian:bookworm-slim
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends xvfb chromium socat supervisor

COPY --link --from=builder /usr/local/bin/main /usr/local/bin/chrome-devtools-protocol-server

COPY supervisord.conf /etc/supervisord.conf

USER 65532

ENTRYPOINT ["supervisord", "-c", "/etc/supervisord.conf"]
