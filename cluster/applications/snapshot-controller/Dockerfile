# syntax=docker/dockerfile:1.4

FROM golang:1.24-bookworm AS builder

ENV CGO_ENABLED=0

WORKDIR /opt/builder

COPY go.mod go.sum /opt/builder/
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY main.go /opt/builder/main.go
COPY bin /opt/builder/bin
COPY api /opt/builder/api
COPY internal /opt/builder/internal

ARG LD_FLAGS="-s -w"
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/snapshot-controller -ldflags="${LD_FLAGS}" /opt/builder/*.go
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/snapshot-capture -ldflags="${LD_FLAGS}" /opt/builder/bin/capture/main.go
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/snapshot-diff -ldflags="${LD_FLAGS}" /opt/builder/bin/diff/main.go
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/snapshot-worker -ldflags="${LD_FLAGS}" /opt/builder/bin/worker/main.go
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -o /usr/local/bin/snapshot-diff-server -ldflags="${LD_FLAGS}" /opt/builder/bin/diff-server/main.go

FROM golang:1.24-bookworm AS snapshot-capture

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2 tini xvfb xauth

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/ms-playwright-go/1.52.0,sharing=locked --mount=type=cache,target=/root/.cache/ms-playwright,sharing=locked go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0 install chromium && cp -r /root/.cache/ms-playwright-go/1.52.0 /usr/local/share/ms-playwright-go && cp -r /root/.cache/ms-playwright /usr/local/share/ms-playwright
ENV PLAYWRIGHT_BROWSERS_PATH="/usr/local/share/ms-playwright"
ENV PLAYWRIGHT_DRIVER_PATH="/usr/local/share/ms-playwright-go"
RUN chmod +x ${PLAYWRIGHT_DRIVER_PATH}/node

COPY --link --from=builder /usr/local/bin/snapshot-capture /usr/local/bin/snapshot-capture

USER 65532

ENV XDG_CONFIG_HOME=/tmp/.chromium
ENV XDG_CACHE_HOME=/tmp/.chromium
ENV DISPLAY=:0
ENTRYPOINT ["/usr/bin/tini", "--", "xvfb-run", "/usr/local/bin/snapshot-capture"]

FROM gcr.io/distroless/static:nonroot AS snapshot-diff
COPY --link --from=builder /usr/local/bin/snapshot-diff /usr/local/bin/snapshot-diff

USER 65532

ENTRYPOINT ["/usr/local/bin/snapshot-diff"]

FROM gcr.io/distroless/static:nonroot AS snapshot-diff-server
COPY --link --from=builder /usr/local/bin/snapshot-diff-server /usr/local/bin/snapshot-diff-server

USER 65532

ENTRYPOINT ["/usr/local/bin/snapshot-diff-server"]

FROM golang:1.24-bookworm AS snapshot-worker

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2 tini xvfb xauth

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/ms-playwright-go/1.52.0,sharing=locked --mount=type=cache,target=/root/.cache/ms-playwright,sharing=locked go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0 install chromium && cp -r /root/.cache/ms-playwright-go/1.52.0 /usr/local/share/ms-playwright-go && cp -r /root/.cache/ms-playwright /usr/local/share/ms-playwright
ENV PLAYWRIGHT_BROWSERS_PATH="/usr/local/share/ms-playwright"
ENV PLAYWRIGHT_DRIVER_PATH="/usr/local/share/ms-playwright-go"
RUN chmod +x ${PLAYWRIGHT_DRIVER_PATH}/node

COPY --link --from=builder /usr/local/bin/snapshot-worker /usr/local/bin/snapshot-worker

USER 65532

ENV XDG_CONFIG_HOME=/tmp/.chromium
ENV XDG_CACHE_HOME=/tmp/.chromium
ENV DISPLAY=:0
ENTRYPOINT ["/usr/bin/tini", "--", "xvfb-run", "/usr/local/bin/snapshot-worker"]

FROM golang:1.24-bookworm

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2 tini xvfb xauth

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/ms-playwright-go/1.52.0,sharing=locked --mount=type=cache,target=/root/.cache/ms-playwright,sharing=locked go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0 install chromium && cp -r /root/.cache/ms-playwright-go/1.52.0 /usr/local/share/ms-playwright-go && cp -r /root/.cache/ms-playwright /usr/local/share/ms-playwright
ENV PLAYWRIGHT_BROWSERS_PATH="/usr/local/share/ms-playwright"
ENV PLAYWRIGHT_DRIVER_PATH="/usr/local/share/ms-playwright-go"
RUN chmod +x ${PLAYWRIGHT_DRIVER_PATH}/node

COPY --link --from=builder /usr/local/bin/snapshot-controller /usr/local/bin/snapshot-controller

USER 65532

ENV XDG_CONFIG_HOME=/tmp/.chromium
ENV XDG_CACHE_HOME=/tmp/.chromium
ENV DISPLAY=:0
ENTRYPOINT ["/usr/bin/tini", "--", "xvfb-run", "/usr/local/bin/snapshot-controller"]
