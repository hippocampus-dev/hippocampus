# syntax=docker/dockerfile:1.4

FROM ghcr.io/hippocampus-dev/hippocampus/ghelpers/ghadapter:main AS ghadapter

FROM ghcr.io/hippocampus-dev/hippocampus/snapshot-controller/snapshot-diff:main
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

COPY --link --from=ghadapter /usr/local/bin/ghadapter /usr/local/bin/ghadapter

USER 1001

ENTRYPOINT ["/usr/local/bin/ghadapter", "/usr/local/bin/snapshot-diff"]
