# syntax=docker/dockerfile:1.4

FROM ghcr.io/rust-lang/rust:nightly-bookworm-slim AS builder

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl gnupg make pkg-config libelf-dev bpftool && \
    curl -fsSL https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add - && \
    echo "deb http://apt.llvm.org/bookworm/ llvm-toolchain-bookworm-20 main" > /etc/apt/sources.list.d/clang.list && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends clang-20 && \
    ln -s /usr/bin/clang-20 /usr/bin/clang

WORKDIR /opt

RUN USER=root cargo new builder

WORKDIR /opt/builder

RUN bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h

# 1. Install rust toolchains first
COPY rust-toolchain.toml /opt/builder/

# Invalid cross-device link
#RUN --mount=type=cache,target=/usr/local/rustup/toolchains rustup show
RUN rustup show
RUN rustup component add rustfmt

# 2. Download dependencies
COPY Cargo.toml Cargo.lock /opt/builder/

RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch && \
      rm -rf /opt/builder/src

# 3. Compile
COPY . /opt/builder

RUN --mount=type=cache,target=/usr/local/cargo/registry --mount=type=cache,target=/opt/builder/target cargo build --release && mv /opt/builder/target/release/connectracer /usr/local/bin/connectracer

# RUSTFLAGS="-C target-feature=+crt-static" is now unstable
#FROM gcr.io/distroless/cc:nonroot
# Install libelf-dev
FROM debian:bookworm-slim
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
ENV RUST_BACKTRACE=1
RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends libelf-dev
COPY --link --from=builder /usr/local/bin/connectracer /usr/local/bin/connectracer

ENTRYPOINT ["/usr/local/bin/connectracer"]
