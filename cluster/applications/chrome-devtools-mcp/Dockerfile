# syntax=docker/dockerfile:1.4

FROM ghcr.io/hippocampus-dev/hippocampus/mcp-stdio-proxy:main AS mcp-stdio-proxy

FROM node:22-slim

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends tini xvfb xauth chromium

RUN --mount=type=cache,target=/root/.npm \
    npm install -g chrome-devtools-mcp@latest

COPY --link --from=mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy

ENV XDG_CONFIG_HOME=/tmp/.chromium
ENV XDG_CACHE_HOME=/tmp/.chromium
ENV DISPLAY=:0
ENV CHROME_PATH=/usr/bin/chromium

USER 65532

ENTRYPOINT ["/usr/bin/tini", "--", "xvfb-run", "/usr/local/bin/mcp-stdio-proxy"]
CMD ["npx", "chrome-devtools-mcp@latest", "--headless"]
