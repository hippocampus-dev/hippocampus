# syntax=docker/dockerfile:1.4

FROM node:22-bookworm-slim AS builder

WORKDIR /opt/builder

COPY package.json package-lock.json /opt/builder/
RUN --mount=type=cache,target=/root/.npm npm ci

FROM gcr.io/distroless/nodejs22:nonroot

COPY --link --from=builder /opt/builder/node_modules /opt/libsodium-encryptor/node_modules
COPY main.mjs /opt/libsodium-encryptor/

USER 65532

CMD ["/opt/libsodium-encryptor/main.mjs"]
