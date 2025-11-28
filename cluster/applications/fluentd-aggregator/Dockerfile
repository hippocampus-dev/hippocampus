# syntax=docker/dockerfile:1.4

FROM fluent/fluentd:v1.16

USER root

RUN gem install fluent-plugin-prometheus -v 2.0.3 && gem install fluent-plugin-grafana-loki -v 1.2.20 && gem install fluent-plugin-elasticsearch -v 5.4.3

USER fluent

COPY plugins /fluentd/plugins

WORKDIR /fluentd
