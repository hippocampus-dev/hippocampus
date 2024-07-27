# syntax=docker/dockerfile:1.4

FROM golang:1.21-bullseye AS builder

ENV CGO_ENABLED 0

WORKDIR /build/

COPY go.mod go.sum /build/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY main.go /build/main.go
COPY api /build/api
COPY internal /build/internal

RUN --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/main -ldflags="-s -w" /build/main.go

FROM gcr.io/distroless/static:nonroot
COPY --link --from=builder /usr/local/bin/main /usr/local/bin/github-actions-runner-controller
COPY --link --from=builder /bin/sleep /bin/sleep

USER 65532

ENTRYPOINT ["/usr/local/bin/github-actions-runner-controller"]
