# syntax=docker/dockerfile:1.4

FROM golang:1.24-bullseye AS builder

ENV CGO_ENABLED=0

WORKDIR /opt/builder

COPY main.go /opt/builder/main.go
ARG LD_FLAGS="-s -w"
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/main -ldflags="${LD_FLAGS}" /opt/builder/*.go

FROM gcr.io/distroless/static:nonroot
COPY --link --from=builder /usr/local/bin/main /usr/local/bin/ghadapter

USER 65532

ENTRYPOINT ["/usr/local/bin/ghadapter"]
