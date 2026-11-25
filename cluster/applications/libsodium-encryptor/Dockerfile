# syntax=docker/dockerfile:1.4

FROM node:26-bookworm-slim AS builder

WORKDIR /opt/builder

COPY package.json package-lock.json .npmrc /opt/builder/
RUN --mount=type=cache,target=/root/.npm npm ci --omit=dev

FROM gcr.io/distroless/nodejs26:nonroot
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

COPY --link --from=builder /opt/builder/node_modules /opt/libsodium-encryptor/node_modules
COPY main.mjs /opt/libsodium-encryptor/

USER 65532

CMD ["/opt/libsodium-encryptor/main.mjs"]
