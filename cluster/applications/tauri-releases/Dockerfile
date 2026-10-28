# syntax=docker/dockerfile:1.4

FROM openresty/openresty:1.27.1.2-bookworm AS builder

ADD https://github.com/zmartzone/lua-resty-openidc/archive/refs/tags/v1.9.2.tar.gz /opt/builder/lua-resty-openidc.tar.gz
ADD https://github.com/ledgetech/lua-resty-http/archive/refs/tags/v0.18.0.tar.gz /opt/builder/lua-resty-http.tar.gz
ADD https://github.com/bungle/lua-resty-session/archive/refs/tags/v4.2.0.tar.gz /opt/builder/lua-resty-session.tar.gz
ADD https://github.com/cdbattags/lua-resty-jwt/archive/refs/tags/v0.4.0.tar.gz /opt/builder/lua-resty-jwt.tar.gz
ADD https://github.com/fffonion/lua-resty-openssl/archive/refs/tags/1.9.0.tar.gz /opt/builder/lua-resty-openssl.tar.gz
ADD https://github.com/hamishforbes/lua-ffi-zlib/archive/refs/tags/v0.6.tar.gz /opt/builder/lua-ffi-zlib.tar.gz

WORKDIR /opt/builder

RUN sha256sum -c <<EOS
96e0601ee6ac8df9b7c2150e7e70da7c257eae6a8fc4deb58d2cc44489f3b4d7  lua-resty-openidc.tar.gz
d84e87bfea7bc1b91c52a27c9464d1412823ce748fd13d3885308f21166f946b  lua-resty-http.tar.gz
4cda4eb3885175657fa3124db7b8e7020d78975e0db403d1135c5a2a736cad47  lua-resty-session.tar.gz
1a3ba164c8f62bebddc6d9a44630313ac39e9c8da503ce3ef7a6a4c3918a5a1c  lua-resty-jwt.tar.gz
d074ac5e6a2bc9c1bd57786b8d5784b6d13681d2fdc4dd7b215eb20f0bd6189c  lua-resty-openssl.tar.gz
151055d1c9f7202a49b4583009be8a60adb3221d8c0166384593db4563465f8c  lua-ffi-zlib.tar.gz
EOS
RUN mkdir -p /opt/builder/lualib && for archive in *.tar.gz; do tar -xzf "$archive" --strip-components=2 -C /opt/builder/lualib --wildcards "*/lib/"; done

FROM openresty/openresty:1.27.1.2-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

COPY --from=builder /opt/builder/lualib /usr/local/openresty/site/lualib
COPY lualib /usr/local/openresty/site/lualib
COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf
COPY releases /usr/share/nginx/html/

USER 65532
