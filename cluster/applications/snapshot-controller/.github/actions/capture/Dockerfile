FROM ghcr.io/kaidotio/hippocampus/ghelpers/ghadapter:main AS ghadapter

FROM ghcr.io/kaidotio/hippocampus/snapshot-controller/snapshot-capture:main

COPY --link --from=ghadapter /usr/local/bin/ghadapter /usr/local/bin/ghadapter

USER 1001

ENTRYPOINT ["/usr/bin/tini", "--", "xvfb-run", "/usr/local/bin/ghadapter", "/usr/local/bin/snapshot-capture"]
