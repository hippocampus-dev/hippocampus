# Node.js Dockerfile Pattern

## Pattern Selection

| Condition | Pattern | Example |
|-----------|---------|---------|
| No build step (run directly with `node`) | Standalone | `cluster/applications/libsodium-encryptor/` |
| Build step required (TypeScript, bundler) | Framework | `docker-compose/comfyui/autocomfy/` |

## Standalone Template

For applications that run directly with `node`:

```dockerfile
# syntax=docker/dockerfile:1.4

FROM node:22-bookworm-slim AS builder

WORKDIR /opt/builder

COPY package.json package-lock.json /opt/builder/
RUN --mount=type=cache,target=/root/.npm npm ci

FROM gcr.io/distroless/nodejs22:nonroot
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

COPY --link --from=builder /opt/builder/node_modules /opt/app/node_modules
COPY main.mjs /opt/app/

USER 65532

CMD ["/opt/app/main.mjs"]
```

## Framework Template

For applications with a build step (TanStack Start, Next.js, etc.):

```dockerfile
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

USER 65532

ENV NODE_ENV=production
ENTRYPOINT ["node", ".output/server/index.mjs"]
```

## Key Elements

| Element | Purpose |
|---------|---------|
| `npm ci` | Reproducible installs from lockfile |
| `npm ci --omit=dev` | Production dependencies only in runtime stage |
| `--mount=type=cache,target=/root/.npm` | Cache npm downloads |
| `node:22-slim` | Minimal Node.js image (for framework apps needing node runtime) |
| `gcr.io/distroless/nodejs22:nonroot` | Minimal runtime (for standalone apps) |
| Non-root user creation | Node.js slim images lack nonroot user |

## Runtime Image Selection

| Condition | Runtime Image |
|-----------|---------------|
| Standalone, no system utilities needed | `gcr.io/distroless/nodejs22:nonroot` |
| Standalone, needs system utilities (CLI tools, child_process) | `node:22-bookworm-slim` + manual nonroot user |
| Build step required (TypeScript, bundler) | `node:22-slim` + manual nonroot user |

## Non-Root User Setup

Node.js slim images require manual user creation (same as Python):

```dockerfile
RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd && \
    echo "nonroot:x:65532:" >> /etc/group && \
    mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot
```
