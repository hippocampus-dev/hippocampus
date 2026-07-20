# Python Dockerfile Pattern

## Standard Template

```dockerfile
# syntax=docker/dockerfile:1.4

FROM python:3.14-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY pyproject.toml uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

FROM python:3.14-slim-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

COPY --link --from=builder /opt/builder/requirements.txt /opt/app/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --upgrade --no-deps -r /opt/app/requirements.txt

COPY main.py /opt/app/main.py
COPY app /opt/app/app

USER 65532

ENV PYTHONPATH="/opt/app"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/app/main.py"]
```

## Key Elements

| Element | Purpose |
|---------|---------|
| `python:3.14-slim-bookworm` | Minimal Debian-based image |
| `uv export --frozen` | Export locked dependencies |
| `--no-deps` | Install without resolving dependencies |
| Non-root user creation | Python images lack nonroot user |
| `PYTHONUNBUFFERED=1` | Immediate log output |

## Non-Root User Setup

Python slim images require manual user creation:

```dockerfile
RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot
```

## System Dependencies

For applications requiring system packages:

```dockerfile
RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends package-name
```

### C++ Extensions

When a dependency builds a C++ extension from source (e.g. a pybind11 module),
install `g++` and `libc6-dev` and set `ENV CXX=g++` before `pip install`.
The base image's sysconfig reports `CXX=gcc`, so setuptools links the extension
without `-lstdc++`; the build succeeds but libstdc++ ABI symbols stay unresolved
until import time.

```dockerfile
RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends g++ libc6-dev

ENV CXX=g++
```

The runtime stage must also install the shared libraries the extension
links against — `libstdc++6` (C++ ABI runtime) and `libgomp1` (OpenMP),
which the `slim` base omits (same `undefined symbol` failure mode, e.g.
`_ZTVN10__cxxabiv120__function_type_infoE`). This applies to prebuilt
native wheels too, not only extensions built from source above.

```dockerfile
RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends libgomp1 libstdc++6
```
