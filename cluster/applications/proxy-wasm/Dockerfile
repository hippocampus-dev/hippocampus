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

RUN USER=root cargo new --lib cookie-manipulator && \
      USER=root cargo new --lib envoy-trusted-header && \
      USER=root cargo new --lib envoy-xauthrequest && \
      USER=root cargo new --lib fallback-filter && \
      USER=root cargo new --lib header-getter && \
      USER=root cargo new --lib header-setter && \
      USER=root cargo new --lib metrics && \
      USER=root cargo new --lib metrics-exporter && \
      USER=root cargo new --lib finalizer

COPY Cargo.toml Cargo.lock /opt/builder/
COPY packages/cookie-manipulator/Cargo.toml /opt/builder/packages/cookie-manipulator/
COPY packages/envoy-trusted-header/Cargo.toml /opt/builder/packages/envoy-trusted-header/
COPY packages/envoy-xauthrequest/Cargo.toml /opt/builder/packages/envoy-xauthrequest/
COPY packages/fallback-filter/Cargo.toml /opt/builder/packages/fallback-filter/
COPY packages/header-getter/Cargo.toml /opt/builder/packages/header-getter/
COPY packages/header-setter/Cargo.toml /opt/builder/packages/header-setter/
COPY packages/metrics/Cargo.toml /opt/builder/packages/metrics/
COPY packages/metrics-exporter/Cargo.toml /opt/builder/packages/metrics-exporter/
COPY packages/finalizer/Cargo.toml /opt/builder/packages/finalizer/

RUN --mount=type=cache,target=/usr/local/cargo/registry cargo fetch && \
      rm -rf /opt/builder/src && \
      rm -rf /opt/builder/packages/cookie-manipulator/src && \
      rm -rf /opt/builder/packages/envoy-trusted-header/src && \
      rm -rf /opt/builder/packages/envoy-xauthrequest/src && \
      rm -rf /opt/builder/packages/fallback-filter/src && \
      rm -rf /opt/builder/packages/header-getter/src && \
      rm -rf /opt/builder/packages/header-setter/src && \
      rm -rf /opt/builder/packages/metrics/src && \
      rm -rf /opt/builder/packages/metrics-exporter/src && \
      rm -rf /opt/builder/packages/finalizer/src

# 3. Compile
COPY . /opt/builder

RUN --mount=type=cache,target=/usr/local/cargo/registry --mount=type=cache,target=/opt/builder/target cargo build --target=wasm32-unknown-unknown --release && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/cookie_manipulator.wasm /usr/local/lib/cookie_manipulator.wasm && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/envoy_trusted_header.wasm /usr/local/lib/envoy_trusted_header.wasm && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/envoy_xauthrequest.wasm /usr/local/lib/envoy_xauthrequest.wasm && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/fallback_filter.wasm /usr/local/lib/fallback_filter.wasm && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/header_getter.wasm /usr/local/lib/header_getter.wasm && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/header_setter.wasm /usr/local/lib/header_setter.wasm && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/metrics_exporter.wasm /usr/local/lib/metrics_exporter.wasm && \
    mv /opt/builder/target/wasm32-unknown-unknown/release/finalizer.wasm /usr/local/lib/finalizer.wasm

RUN <<EOS
cat <<EOSS > /usr/local/share/index.html
<!doctype html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport"
          content="width=device-width, user-scalable=no, initial-scale=1.0, maximum-scale=1.0, minimum-scale=1.0">
    <meta http-equiv="X-UA-Compatible" content="ie=edge">
    <title>proxy-wasm</title>
</head>
<body>
    <ul>
        <li><a href="/cookie_manipulator.wasm">cookie_manipulator.wasm</a>: $(sha256sum /usr/local/lib/cookie_manipulator.wasm | cut -d' ' -f1)</li>
        <li><a href="/envoy_trusted_header.wasm">envoy_trusted_header.wasm</a>: $(sha256sum /usr/local/lib/envoy_trusted_header.wasm | cut -d' ' -f1)</li>
        <li><a href="/envoy_xauthrequest.wasm">envoy_xauthrequest.wasm</a>: $(sha256sum /usr/local/lib/envoy_xauthrequest.wasm | cut -d' ' -f1)</li>
        <li><a href="/fallback_filter.wasm">fallback_filter.wasm</a>: $(sha256sum /usr/local/lib/fallback_filter.wasm | cut -d' ' -f1)</li>
        <li><a href="/header_getter.wasm">header_getter.wasm</a>: $(sha256sum /usr/local/lib/header_getter.wasm | cut -d' ' -f1)</li>
        <li><a href="/header_setter.wasm">header_setter.wasm</a>: $(sha256sum /usr/local/lib/header_setter.wasm | cut -d' ' -f1)</li>
        <li><a href="/metrics_exporter.wasm">metrics_exporter.wasm</a>: $(sha256sum /usr/local/lib/metrics_exporter.wasm | cut -d' ' -f1)</li>
        <li><a href="/finalizer.wasm">finalizer.wasm</a>: $(sha256sum /usr/local/lib/finalizer.wasm | cut -d' ' -f1)</li>
    </ul>
</body>
</html>
EOSS
EOS

FROM scratch AS cookie-manipulator
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/cookie_manipulator.wasm /plugin.wasm

FROM scratch AS envoy-trusted-header
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/envoy_trusted_header.wasm /plugin.wasm

FROM scratch AS envoy-xauthrequest
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/envoy_xauthrequest.wasm /plugin.wasm

FROM scratch AS fallback-filter
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/fallback_filter.wasm /plugin.wasm

FROM scratch AS header-getter
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/header_getter.wasm /plugin.wasm

FROM scratch AS header-setter
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/header_setter.wasm /plugin.wasm

FROM scratch AS metrics-exporter
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/metrics_exporter.wasm /plugin.wasm

FROM scratch AS finalizer
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=builder /usr/local/lib/finalizer.wasm /plugin.wasm

FROM ghcr.io/hippocampus-dev/hippocampus/nghttp2:main AS server
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=cookie-manipulator /plugin.wasm /usr/local/share/nghttp2/cookie_manipulator.wasm
COPY --link --from=envoy-trusted-header /plugin.wasm /usr/local/share/nghttp2/envoy_trusted_header.wasm
COPY --link --from=envoy-xauthrequest /plugin.wasm /usr/local/share/nghttp2/envoy_xauthrequest.wasm
COPY --link --from=fallback-filter /plugin.wasm /usr/local/share/nghttp2/fallback_filter.wasm
COPY --link --from=header-getter /plugin.wasm /usr/local/share/nghttp2/header_getter.wasm
COPY --link --from=header-setter /plugin.wasm /usr/local/share/nghttp2/header_setter.wasm
COPY --link --from=metrics-exporter /plugin.wasm /usr/local/share/nghttp2/metrics_exporter.wasm
COPY --link --from=finalizer /plugin.wasm /usr/local/share/nghttp2/finalizer.wasm
COPY --link --from=builder /usr/local/share/index.html /usr/local/share/nghttp2/index.html

ENTRYPOINT ["nghttpd", "8080", "--address=0.0.0.0", "--no-tls", "--htdocs=/usr/local/share/nghttp2"]

FROM envoyproxy/envoy:v1.24.0
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"
COPY --link --from=cookie-manipulator /plugin.wasm /cookie_manipulator.wasm
COPY --link --from=envoy-trusted-header /plugin.wasm /envoy_trusted_header.wasm
COPY --link --from=envoy-xauthrequest /plugin.wasm /envoy_xauthrequest.wasm
COPY --link --from=fallback-filter /plugin.wasm /fallback_filter.wasm
COPY --link --from=header-getter /plugin.wasm /header_getter.wasm
COPY --link --from=header-setter /plugin.wasm /header_setter.wasm
COPY --link --from=metrics-exporter /plugin.wasm /metrics_exporter.wasm
COPY --link --from=finalizer /plugin.wasm /finalizer.wasm
