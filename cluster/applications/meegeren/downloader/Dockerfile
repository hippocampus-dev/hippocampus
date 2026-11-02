# syntax=docker/dockerfile:1.4

FROM debian:bookworm-slim
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends aria2 ca-certificates

COPY --chown=nonroot:nonroot links.txt /var/aria2c/links.txt
COPY --chown=nonroot:nonroot gated-links.txt /var/aria2c/gated-links.txt

USER 65532

# The input file is chosen per run, since only gated-links.txt may be sent the Hugging Face token
ENTRYPOINT ["aria2c", "-d", "/home/nonroot", "--continue", "--auto-file-renaming", "false", "--allow-overwrite", "true", "-j", "5", "-x", "8", "-s", "8", "-k", "100M"]
CMD ["-i", "/var/aria2c/links.txt"]
