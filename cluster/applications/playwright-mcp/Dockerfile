FROM ghcr.io/hippocampus-dev/hippocampus/mcp-stdio-proxy:main AS mcp-stdio-proxy

FROM mcr.microsoft.com/playwright/mcp:v.0.0.26

USER root

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends tini xvfb xauth

USER node

COPY --link --from=mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy

ENV XDG_CONFIG_HOME=/tmp/.chromium
ENV XDG_CACHE_HOME=/tmp/.chromium
ENV DISPLAY=:0

ENTRYPOINT ["/usr/bin/tini", "--", "xvfb-run", "/usr/local/bin/mcp-stdio-proxy"]
CMD ["node", "cli.js", "--headless", "--browser", "chromium", "--no-sandbox"]
