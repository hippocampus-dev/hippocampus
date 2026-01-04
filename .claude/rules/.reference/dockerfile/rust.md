# Rust Dockerfile Pattern

## Standard Template

```dockerfile
# syntax=docker/dockerfile:1.4

FROM rust:1.87-bookworm AS builder

WORKDIR /opt/builder

# 1. Copy toolchain config first
COPY rust-toolchain.toml /opt/builder/

# 2. Download dependencies
COPY Cargo.toml Cargo.lock /opt/builder/
RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch

# 3. Compile
COPY src /opt/builder/src
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/opt/builder/target \
    cargo build --release && \
    mv /opt/builder/target/release/app-name /usr/local/bin/app-name

FROM gcr.io/distroless/cc:nonroot
ENV RUST_BACKTRACE=1
COPY --link --from=builder /usr/local/bin/app-name /usr/local/bin/app-name

USER 65532

ENTRYPOINT ["/usr/local/bin/app-name"]
```

## Key Elements

| Element | Purpose |
|---------|---------|
| `rust:1.87-bookworm` | Debian-based Rust image |
| `rust-toolchain.toml` | Pin Rust version |
| `cargo fetch` | Download dependencies separately |
| `--mount=type=cache,target=/usr/local/cargo/registry` | Cache downloaded crates |
| `--mount=type=cache,target=/opt/builder/target` | Cache build artifacts |
| `gcr.io/distroless/cc:nonroot` | Minimal runtime with libc |
| `USER 65532` | Non-root user |
| `ENV RUST_BACKTRACE=1` | Enable backtrace on panic |

## Protobuf/gRPC Applications

For applications needing `protoc` and static linking:

```dockerfile
# syntax=docker/dockerfile:1.4

FROM ghcr.io/rust-lang/rust:nightly-bookworm-slim AS builder

# Install protoc
ENV PROTOBUF_VERSION=21.11
RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends curl unzip && \
    curl -fsSL https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOBUF_VERSION}/protoc-${PROTOBUF_VERSION}-linux-x86_64.zip -o /tmp/protoc.zip && \
    unzip -o /tmp/protoc.zip -d /usr/local && \
    rm /tmp/protoc.zip

WORKDIR /opt/builder
COPY rust-toolchain.toml /opt/builder/
COPY Cargo.toml Cargo.lock /opt/builder/
RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch

COPY src /opt/builder/src
COPY proto /opt/builder/proto

# Static linking for distroless/static
ENV RUSTFLAGS "-C target-feature=+crt-static"
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/opt/builder/target \
    cargo build --release && \
    mv /opt/builder/target/release/app-name /usr/local/bin/app-name

FROM gcr.io/distroless/static:nonroot
ENV RUST_BACKTRACE=1
COPY --link --from=builder /usr/local/bin/app-name /usr/local/bin/app-name

USER 65532
ENTRYPOINT ["/usr/local/bin/app-name"]
```

## WASM Applications

For Envoy proxy-wasm filters or other WASM artifacts:

```dockerfile
# syntax=docker/dockerfile:1.4

FROM ghcr.io/rust-lang/rust:nightly-bookworm-slim AS builder

WORKDIR /opt/builder
COPY rust-toolchain.toml /opt/builder/
RUN rustup target add wasm32-unknown-unknown

COPY Cargo.toml Cargo.lock /opt/builder/
RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch

COPY src /opt/builder/src
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/opt/builder/target \
    cargo build --target=wasm32-unknown-unknown --release && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/plugin.wasm /plugin.wasm

FROM scratch
COPY --link --from=builder /plugin.wasm /plugin.wasm
```

## eBPF Applications

For eBPF applications requiring kernel headers and clang:

```dockerfile
# syntax=docker/dockerfile:1.4

FROM ghcr.io/rust-lang/rust:nightly-bookworm-slim AS builder

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends \
        curl gnupg make pkg-config libelf-dev bpftool clang

WORKDIR /opt/builder

RUN bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h

COPY rust-toolchain.toml /opt/builder/
RUN rustup show

COPY Cargo.toml Cargo.lock /opt/builder/
RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch

COPY . /opt/builder
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/opt/builder/target \
    cargo build --release && \
    mv /opt/builder/target/release/app-name /usr/local/bin/app-name

FROM debian:bookworm-slim
ENV RUST_BACKTRACE=1
RUN --mount=type=cache,target=/var/cache/apt/archives \
    apt-get update -y && apt-get install -y --no-install-recommends libelf-dev
COPY --link --from=builder /usr/local/bin/app-name /usr/local/bin/app-name

ENTRYPOINT ["/usr/local/bin/app-name"]
```

## Build Caches

Always mount both caches for Rust builds:

```dockerfile
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/opt/builder/target \
    cargo build --release
```

* `/usr/local/cargo/registry` - Downloaded crates
* `/opt/builder/target` - Build artifacts
