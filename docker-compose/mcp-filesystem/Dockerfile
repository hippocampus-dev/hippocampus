FROM ghcr.io/kaidotio/hippocampus/mcp-stdio-proxy:main AS mcp-stdio-proxy

FROM mcp/filesystem:latest

COPY --link --from=mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy

ENTRYPOINT ["/usr/local/bin/mcp-stdio-proxy"]
CMD ["node", "/app/dist/index.js"]
