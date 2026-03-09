# syntax=docker/dockerfile:1.4

FROM rust:1.89-bookworm AS builder

ENV PROTOBUF_VERSION=21.11

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends curl unzip && \
    curl -fsSL https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOBUF_VERSION}/protoc-${PROTOBUF_VERSION}-linux-x86_64.zip -o /tmp/protoc.zip && \
    unzip -o /tmp/protoc.zip -d /usr/local && \
    rm /tmp/protoc.zip

WORKDIR /opt

RUN USER=root cargo new builder

WORKDIR /opt/builder

COPY rust-toolchain.toml /opt/builder/

COPY Cargo.toml Cargo.lock /opt/builder/
RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch && \
    rm -rf /opt/builder/src

COPY src /opt/builder/src
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/opt/builder/target \
    cargo build --release && \
    mv /opt/builder/target/release/envoy-request-hasher /usr/local/bin/envoy-request-hasher

FROM gcr.io/distroless/cc:nonroot
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
ENV RUST_BACKTRACE=1
COPY --link --from=builder /usr/local/bin/envoy-request-hasher /usr/local/bin/envoy-request-hasher

USER 65532

ENTRYPOINT ["/usr/local/bin/envoy-request-hasher"]
