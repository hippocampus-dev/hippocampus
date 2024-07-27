# syntax=docker/dockerfile:1.4

FROM ghcr.io/rust-lang/rust:nightly-bookworm-slim AS builder

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl gnupg make pkg-config libelf-dev bpftool && \
    curl -fsSL https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add - && \
    echo "deb http://apt.llvm.org/bookworm/ llvm-toolchain-bookworm main" > /etc/apt/sources.list.d/clang.list && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends clang

WORKDIR /opt

RUN USER=root cargo new builder

WORKDIR /opt/builder

RUN bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h

# 1. Install rust toolchains first
COPY rust-toolchain.toml /opt/builder/

# Invalid cross-device link
#RUN --mount=type=cache,target=/usr/local/rustup/toolchains rustup show
RUN rustup show

# 2. Download dependencies
COPY Cargo.toml Cargo.lock /opt/builder/

RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch && \
      rm -rf /opt/builder/src

# 3. Compile
COPY . /opt/builder

ARG bin=connectracer

RUN --mount=type=cache,target=/opt/builder/target --mount=type=cache,target=/usr/local/cargo/registry cargo build --release --bin $bin && mv /opt/builder/target/release/$bin /usr/local/bin/main

ENTRYPOINT ["/usr/local/bin/main"]
