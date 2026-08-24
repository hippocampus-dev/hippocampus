# syntax=docker/dockerfile:1.4

FROM golang:1.25-bookworm AS builder

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends librdkafka-dev

WORKDIR /opt/builder

COPY go.mod go.sum /opt/builder/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY main.go /opt/builder/main.go
COPY cmd /opt/builder/cmd
COPY internal /opt/builder/internal
COPY pkg /opt/builder/pkg

ARG LD_FLAGS="-s -w"
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/main -ldflags="${LD_FLAGS}" /opt/builder/*.go

FROM gcr.io/distroless/cc:nonroot
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/bin/main /usr/local/bin/loganomaly

USER 65532

ENTRYPOINT ["/usr/local/bin/loganomaly"]
