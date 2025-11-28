# syntax=docker/dockerfile:1.4

FROM debian:bookworm-slim as builder

ARG NGHTTP2_VERSION=v1.61.0

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git ca-certificates make autoconf automake libtool pkg-config g++ libssl-dev libev-dev libc-ares-dev libjemalloc-dev zlib1g-dev

RUN --mount=type=cache,target=/usr/local/src \
    [ -d /usr/local/src/nghttp2-${NGHTTP2_VERSION} ] || git clone https://github.com/nghttp2/nghttp2 -b $NGHTTP2_VERSION --single-branch --depth 1 /usr/local/src/nghttp2-${NGHTTP2_VERSION} && \
    cd /usr/local/src/nghttp2-${NGHTTP2_VERSION} && \
    autoreconf -i && \
    ./configure --disable-examples --disable-hpack-tools \
    LIBTOOL_LDFLAGS="-static-libtool-libs" \
    OPENSSL_LIBS="-l:libssl.a -l:libcrypto.a" \
    LIBEV_LIBS="-l:libev.a" \
    LIBCARES_LIBS="-l:libcares.a" \
    JEMALLOC_LIBS="-l:libjemalloc.a" \
    ZLIB_LIBS="-l:libz.a" && \
    make -j$(nproc) install-strip

FROM debian:bookworm-slim

COPY --link --from=builder /usr/local/share/nghttp2/ /usr/local/share/nghttp2/
COPY --link --from=builder /usr/local/bin/nghttp /usr/local/bin/nghttp
COPY --link --from=builder /usr/local/bin/nghttpd /usr/local/bin/nghttpd
COPY --link --from=builder /usr/local/bin/nghttpx /usr/local/bin/nghttpx
COPY --link --from=builder /usr/local/bin/h2load /usr/local/bin/h2load
