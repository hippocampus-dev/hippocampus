# syntax=docker/dockerfile:1.4

FROM node:21-bookworm-slim AS builder

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

WORKDIR /opt/libsodium-encryptor

COPY package.json package-lock.json /opt/libsodium-encryptor/

RUN --mount=type=cache,target=/root/.npm npm ci

USER 65532

COPY main.mjs /opt/libsodium-encryptor/

ENTRYPOINT ["node", "main.mjs"]
