# Go Dockerfile Pattern

## Standard Template

```dockerfile
# syntax=docker/dockerfile:1.4

FROM golang:1.24-bullseye AS builder

ENV CGO_ENABLED=0

WORKDIR /opt/builder

COPY go.mod go.sum /opt/builder/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY main.go /opt/builder/main.go
COPY cmd /opt/builder/cmd
COPY internal /opt/builder/internal

ARG LD_FLAGS="-s -w"
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -o /usr/local/bin/main -ldflags="${LD_FLAGS}" /opt/builder/*.go

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/bin/main /usr/local/bin/app-name

USER 65532

ENTRYPOINT ["/usr/local/bin/app-name"]
CMD ["subcommand"]
```

## Key Elements

| Element | Purpose |
|---------|---------|
| `CGO_ENABLED=0` | Static binary (no glibc dependency) |
| `LD_FLAGS="-s -w"` | Strip debug info (smaller binary) |
| `-trimpath` | Reproducible builds |
| `gcr.io/distroless/static:nonroot` | Minimal runtime image |
| `USER 65532` | Non-root user |

## Build Caches

```dockerfile
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build ...
```

Always mount both:
* `/go/pkg/mod` - Module cache
* `/root/.cache/go-build` - Build cache
