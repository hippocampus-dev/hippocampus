# syntax=docker/dockerfile:1.4

FROM ghcr.io/rust-lang/rust:nightly-bookworm-slim AS builder

ENV PROTOBUF_VERSION=21.11

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl unzip && \
    curl -fsSL https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOBUF_VERSION}/protoc-${PROTOBUF_VERSION}-linux-x86_64.zip -o /tmp/protoc.zip && \
    unzip -o /tmp/protoc.zip -d /usr/local && \
    rm /tmp/protoc.zip

WORKDIR /opt

RUN USER=root cargo new builder

WORKDIR /opt/builder

# 1. Install rust toolchains first
COPY rust-toolchain.toml /opt/builder/

# Invalid cross-device link
#RUN --mount=type=cache,target=/usr/local/rustup/toolchains rustup show
RUN rustup show

# 2. Download dependencies
WORKDIR /opt/builder/packages

RUN USER=root cargo new --lib elapsed && \
      USER=root cargo new --lib elapsed_macro && \
      USER=root cargo new --lib elf && \
      USER=root cargo new --lib enum_derive && \
      USER=root cargo new --lib error && \
      USER=root cargo new --lib gcs && \
      USER=root cargo new --lib hedged && \
      USER=root cargo new --lib hippocampus-client && \
      USER=root cargo new --lib hippocampus-configuration && \
      USER=root cargo new --lib hippocampus-core && \
      USER=root cargo new --lib hippocampus-server && \
      USER=root cargo new --lib hippocampus-standalone && \
      USER=root cargo new --lib hippocampus-web && \
      USER=root cargo new --lib hippocampusql && \
      USER=root cargo new --lib husky && \
      USER=root cargo new --lib jwt && \
      USER=root cargo new --lib jwt_derive && \
      USER=root cargo new --lib opentelemetry-tracing && \
      USER=root cargo new --lib retry && \
      USER=root cargo new --lib serde_binary && \
      USER=root cargo new --lib singleflight && \
      USER=root cargo new --lib bakery && \
      USER=root cargo new --lib bloom-filter && \
      USER=root cargo new --lib openai && \
      USER=root cargo new --lib audio && \
      USER=root cargo new --lib mysql-protocol-parser && \
      USER=root cargo new --lib hippocampus-wasm-tokenizer-example

WORKDIR /opt/builder/packages/hippocampus-core

RUN USER=root cargo new --lib benches

WORKDIR /opt/builder/packages/hippocampus-core/examples

RUN USER=root cargo new --lib simple

WORKDIR /opt/builder/packages/gcs/examples

RUN USER=root cargo new --lib http2-server

WORKDIR /opt/builder

COPY Cargo.toml Cargo.lock /opt/builder/
COPY packages/elapsed/Cargo.toml /opt/builder/packages/elapsed/
COPY packages/elapsed_macro/Cargo.toml /opt/builder/packages/elapsed_macro/
COPY packages/elf/Cargo.toml /opt/builder/packages/elf/
COPY packages/enum_derive/Cargo.toml /opt/builder/packages/enum_derive/
COPY packages/error/Cargo.toml /opt/builder/packages/error/
COPY packages/gcs/Cargo.toml /opt/builder/packages/gcs/
COPY packages/gcs/examples/http2-server/Cargo.toml /opt/builder/packages/gcs/examples/http2-server/
COPY packages/hedged/Cargo.toml /opt/builder/packages/hedged/
COPY packages/hippocampus-client/Cargo.toml /opt/builder/packages/hippocampus-client/
COPY packages/hippocampus-configuration/Cargo.toml /opt/builder/packages/hippocampus-configuration/
COPY packages/hippocampus-core/Cargo.toml /opt/builder/packages/hippocampus-core/
COPY packages/hippocampus-core/benches/Cargo.toml /opt/builder/packages/hippocampus-core/benches/
COPY packages/hippocampus-core/examples/simple/Cargo.toml /opt/builder/packages/hippocampus-core/examples/simple/
COPY packages/hippocampus-server/Cargo.toml /opt/builder/packages/hippocampus-server/
COPY packages/hippocampus-standalone/Cargo.toml /opt/builder/packages/hippocampus-standalone/
COPY packages/hippocampus-web/Cargo.toml /opt/builder/packages/hippocampus-web/
COPY packages/hippocampusql/Cargo.toml /opt/builder/packages/hippocampusql/
COPY packages/husky/Cargo.toml /opt/builder/packages/husky/
COPY packages/jwt/Cargo.toml /opt/builder/packages/jwt/
COPY packages/jwt_derive/Cargo.toml /opt/builder/packages/jwt_derive/
COPY packages/opentelemetry-tracing/Cargo.toml /opt/builder/packages/opentelemetry-tracing/
COPY packages/retry/Cargo.toml /opt/builder/packages/retry/
COPY packages/serde_binary/Cargo.toml /opt/builder/packages/serde_binary/
COPY packages/singleflight/Cargo.toml /opt/builder/packages/singleflight/
COPY packages/bakery/Cargo.toml /opt/builder/packages/bakery/
COPY packages/bloom-filter/Cargo.toml /opt/builder/packages/bloom-filter/
COPY packages/openai/Cargo.toml /opt/builder/packages/openai/
COPY packages/audio/Cargo.toml /opt/builder/packages/audio/
COPY packages/mysql-protocol-parser/Cargo.toml /opt/builder/packages/mysql-protocol-parser/
COPY packages/hippocampus-wasm-tokenizer-example/Cargo.toml /opt/builder/packages/hippocampus-wasm-tokenizer-example/

RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch && \
      rm -rf /opt/builder/src && \
      rm -rf /opt/builder/packages/elapsed/src && \
      rm -rf /opt/builder/packages/elapsed_macro/src && \
      rm -rf /opt/builder/packages/elf/src && \
      rm -rf /opt/builder/packages/enum_derive/src && \
      rm -rf /opt/builder/packages/error/src && \
      rm -rf /opt/builder/packages/gcs/src && \
      rm -rf /opt/builder/packages/gcs/examples/http2-server/src && \
      rm -rf /opt/builder/packages/hedged/src && \
      rm -rf /opt/builder/packages/hippocampus-client/src && \
      rm -rf /opt/builder/packages/hippocampus-configuration/src && \
      rm -rf /opt/builder/packages/hippocampus-core/src && \
      rm -rf /opt/builder/packages/hippocampus-core/benches/src && \
      rm -rf /opt/builder/packages/hippocampus-core/examples/simple/src && \
      rm -rf /opt/builder/packages/hippocampus-server/src && \
      rm -rf /opt/builder/packages/hippocampus-standalone/src && \
      rm -rf /opt/builder/packages/hippocampus-web/src && \
      rm -rf /opt/builder/packages/hippocampusql/src && \
      rm -rf /opt/builder/packages/husky/src && \
      rm -rf /opt/builder/packages/jwt/src && \
      rm -rf /opt/builder/packages/jwt_derive/src && \
      rm -rf /opt/builder/packages/opentelemetry-tracing/src && \
      rm -rf /opt/builder/packages/retry/src && \
      rm -rf /opt/builder/packages/serde_binary/src && \
      rm -rf /opt/builder/packages/singleflight/src && \
      rm -rf /opt/builder/packages/bakery/src && \
      rm -rf /opt/builder/packages/bloom-filter/src && \
      rm -rf /opt/builder/packages/openai/src && \
      rm -rf /opt/builder/packages/audio/src && \
      rm -rf /opt/builder/packages/mysql-protocol-parser/src && \
      rm -rf /opt/builder/packages/hippocampus-wasm-tokenizer-example/src

# 3. Compile
COPY packages /opt/builder/packages/
COPY proto /opt/builder/proto/

ARG bin=hippocampus-standalone
ARG features=jemalloc

ENV RUSTFLAGS "-C target-feature=+crt-static"
RUN --mount=type=cache,target=/usr/local/cargo/registry --mount=type=cache,target=/opt/builder/target cargo build --release --bin $bin --features $features --target x86_64-unknown-linux-gnu && mv /opt/builder/target/x86_64-unknown-linux-gnu/release/$bin /usr/local/bin/main

ENTRYPOINT ["/usr/local/bin/main"]

FROM gcr.io/distroless/static:nonroot
ENV RUST_BACKTRACE=1
COPY --link --from=builder /usr/local/bin/main /usr/local/bin/hippocampus

USER 65532

ENTRYPOINT ["/usr/local/bin/hippocampus"]
