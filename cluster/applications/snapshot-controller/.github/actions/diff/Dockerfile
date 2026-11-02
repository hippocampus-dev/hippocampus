FROM ghcr.io/hippocampus-dev/hippocampus/ghelpers/ghadapter:main AS ghadapter

FROM ghcr.io/hippocampus-dev/hippocampus/snapshot-controller/snapshot-diff:main

COPY --link --from=ghadapter /usr/local/bin/ghadapter /usr/local/bin/ghadapter

USER 1001

ENTRYPOINT ["/usr/local/bin/ghadapter", "/usr/local/bin/snapshot-diff"]
