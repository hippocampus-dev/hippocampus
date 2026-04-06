# syntax=docker/dockerfile:1.4

FROM debian:bookworm-slim
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/bin/bash" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends supervisor nginx tmux

ARG TTYD_VERSION=1.7.7
ADD --chmod=755 https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.x86_64 /usr/local/bin/ttyd

ARG CHISEL_VERSION=1.11.5
ADD --chmod=644 https://github.com/jpillora/chisel/releases/download/v${CHISEL_VERSION}/chisel_${CHISEL_VERSION}_linux_amd64.gz /tmp/chisel.gz
RUN gunzip /tmp/chisel.gz && install -m 755 /tmp/chisel /usr/local/bin/chisel && rm /tmp/chisel

COPY supervisord.conf /etc/supervisord.conf
COPY nginx.conf /etc/nginx/nginx.conf
COPY --chmod=755 entrypoint.sh /usr/local/bin/entrypoint.sh

RUN mkdir -p /tmp/nginx && chown nonroot:nonroot /tmp/nginx

USER 65532

ENTRYPOINT ["entrypoint.sh"]
