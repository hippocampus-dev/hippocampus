FROM debian:bookworm-slim

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends aria2

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

USER 65532

WORKDIR /home/nonroot

COPY --chown=nonroot:nonroot links.txt links.txt

ENTRYPOINT ["aria2c", "-i=links.txt"]
CMD ["-j=5", "-x=8", "-s=8", "-k=100M", "-d=.", "--continue"]

