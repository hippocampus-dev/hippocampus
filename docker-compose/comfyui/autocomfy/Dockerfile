# syntax=docker/dockerfile:1.4

FROM node:22-slim AS builder

WORKDIR /opt/builder

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . .
RUN npm run build

FROM node:22-slim
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd && \
    echo "nonroot:x:65532:" >> /etc/group && \
    mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

WORKDIR /opt/app

COPY --link --from=builder /opt/builder/package.json /opt/builder/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --omit=dev

COPY --link --from=builder /opt/builder/.output ./.output

RUN mkdir -p /opt/app/data/labels && chown -R nonroot:nonroot /opt/app/data

USER 65532

ENV NODE_ENV=production
ENTRYPOINT ["node", ".output/server/index.mjs"]
