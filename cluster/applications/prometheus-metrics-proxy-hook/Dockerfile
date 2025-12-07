# syntax=docker/dockerfile:1.4

FROM golang:1.24-bullseye AS builder

ENV CGO_ENABLED=0

WORKDIR /opt/builder

COPY go.mod go.sum /opt/builder/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY main.go /opt/builder/main.go
COPY cmd /opt/builder/cmd
COPY pkg /opt/builder/pkg

ARG LD_FLAGS="-s -w"
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/main -ldflags="${LD_FLAGS}" /opt/builder/*.go

FROM gcr.io/distroless/static:nonroot
COPY --link --from=builder /usr/local/bin/main /usr/local/bin/prometheus-metrics-proxy-hook

USER 65532

ENTRYPOINT ["/usr/local/bin/prometheus-metrics-proxy-hook"]
CMD ["webhook"]
