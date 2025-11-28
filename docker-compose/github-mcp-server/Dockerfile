FROM ghcr.io/kaidotio/hippocampus/mcp-stdio-proxy:main AS mcp-stdio-proxy

FROM ghcr.io/github/github-mcp-server:v0.1.1

COPY --link --from=mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy /usr/local/bin/mcp-stdio-proxy

ENTRYPOINT ["/usr/local/bin/mcp-stdio-proxy"]
CMD ["/server/github-mcp-server", "stdio"]
