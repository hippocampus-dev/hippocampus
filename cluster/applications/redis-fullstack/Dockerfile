# syntax=docker/dockerfile:1.4

FROM falkordb/falkordb:v4.8.6 AS falkordb

FROM redis/redis-stack:7.4.0-v0
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends libgomp1 && \
    rm -rf /var/lib/apt/lists/*

COPY --link --from=falkordb /FalkorDB/bin/src/falkordb.so /opt/redis-stack/lib/falkordb.so
