# syntax=docker/dockerfile:1.4

FROM ghcr.io/rust-lang/rust:nightly-bookworm-slim AS builder

WORKDIR /opt

RUN USER=root cargo new builder

WORKDIR /opt/builder

# 1. Install rust toolchains first
COPY rust-toolchain.toml /opt/builder/

# Invalid cross-device link
#RUN --mount=type=cache,target=/usr/local/rustup/toolchains rustup show
RUN rustup show

RUN rustup target add wasm32-unknown-unknown

# 2. Download dependencies
WORKDIR /opt/builder/packages

RUN USER=root cargo new --lib header-debugger

WORKDIR /opt/builder

COPY Cargo.toml Cargo.lock /opt/builder/
COPY packages/header-debugger/Cargo.toml /opt/builder/packages/header-debugger/

RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch && \
      rm -rf /opt/builder/src && \
      rm -rf /opt/builder/packages/header-debugger/src

# 3. Compile
COPY packages /opt/builder/packages/

RUN --mount=type=cache,target=/opt/builder/target --mount=type=cache,target=/usr/local/cargo/registry cargo build --target=wasm32-unknown-unknown --release && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/header_debugger.wasm /usr/local/lib/header_debugger.wasm

FROM envoyproxy/envoy:v1.24.0
COPY --link --from=builder /usr/local/lib/header_debugger.wasm /header_debugger.wasm
COPY envoy.yaml /etc/envoy/envoy.yaml
