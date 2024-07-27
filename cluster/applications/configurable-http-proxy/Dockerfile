FROM node:21-bookworm-slim AS builder

WORKDIR /opt/builder
RUN npm init -y
RUN npm install configurable-http-proxy-redis-backend@0.1.6

FROM quay.io/jupyterhub/configurable-http-proxy:4.6.1

COPY --link --from=builder /opt/builder/node_modules node_modules
