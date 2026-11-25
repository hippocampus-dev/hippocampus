# syntax=docker/dockerfile:1.4

FROM ghcr.io/hippocampus-dev/hippocampus/mcp-stdio-proxy:main AS mcp-stdio-proxy

FROM mcp/filesystem:latest
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

COPY --link --from=mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy

ENTRYPOINT ["/usr/local/bin/mcp-stdio-proxy"]
CMD ["node", "/app/dist/index.js"]
