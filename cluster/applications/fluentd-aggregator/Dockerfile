# syntax=docker/dockerfile:1.4

FROM fluent/fluentd:v1.16
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

USER root

COPY patches /tmp/patches
RUN apk add --no-cache patch && \
    apk add --no-cache --virtual .build-deps ruby-dev build-base && \
    gem install fluent-plugin-record-modifier -v 2.2.1 && \
    gem install fluent-plugin-prometheus -v 2.0.3 && \
    gem install fluent-plugin-grafana-loki -v 1.2.20 && \
    gem install fluent-plugin-elasticsearch -v 5.4.3 && \
    gem install fluent-plugin-kafka -v 0.19.4 && \
    patch --fuzz=0 -p1 -d /usr/lib/ruby/gems/*/gems/fluent-plugin-grafana-loki-1.2.20 < /tmp/patches/out_loki_unrecoverable_error.patch && \
    rm -rf /tmp/patches && \
    apk del .build-deps patch

USER fluent

COPY plugins /fluentd/plugins

WORKDIR /fluentd
