# syntax=docker/dockerfile:1.4

FROM node:22-bookworm-slim

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2

WORKDIR /opt/lighthouse-exporter

ENV PUPPETEER_CACHE_DIR="/usr/local/share/puppeteer"

COPY package.json package-lock.json /opt/lighthouse-exporter/
RUN --mount=type=cache,target=/root/.npm npm ci

COPY main.mjs /opt/lighthouse-exporter/

USER 65532

ENTRYPOINT ["node", "/opt/lighthouse-exporter/main.mjs"]
