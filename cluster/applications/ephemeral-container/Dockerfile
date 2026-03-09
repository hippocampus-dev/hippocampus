# syntax=docker/dockerfile:1.4

FROM ubuntu:24.04
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

ARG DEBIAN_FRONTEND=noninteractive

ENV MC_HOST_minio=http://minio:miniominio@127.0.0.1:9000
ENV AWS_ACCESS_KEY_ID=minio
ENV AWS_SECRET_ACCESS_KEY=miniominio
ENV S3_ENDPOINT_URL=http://127.0.0.1:9000
ENV LOKI_ADDR=http://127.0.0.1:3100
ENV MIMIR_ADDRESS=http://127.0.0.1:3100
ENV MIMIR_TENANT_ID=anonymous
ENV MEMCACHED_SERVERS=127.0.0.1:11211

ENV PROCPS_VERSION=v4.0.4
ENV CRICTL_VERSION=v1.26.0

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git automake ruby ruby-dev nodejs npm python3 python3-pip sudo gcc make autoconf libtool gettext autopoint pkg-config libncurses5-dev unzip apt-transport-https ca-certificates gnupg vim lsof strace tcpdump tshark netcat-traditional net-tools curl nghttp2 jq dnsutils iputils-ping iproute2 redis-tools libmemcached-tools mysql-client postgresql-client bc kcat

ADD --chmod=700 https://dl.min.io/client/mc/release/linux-amd64/mc /usr/local/bin/mc
ADD --chmod=700 https://github.com/grafana/mimir/releases/download/mimir-2.11.0/mimirtool-linux-amd64 /usr/local/bin/mimirtool

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked --mount=type=cache,target=/usr/local/src \
    [ -d /usr/local/src/procps-${PROCPS_VERSION} ] || git clone https://gitlab.com/procps-ng/procps.git -b ${PROCPS_VERSION} --single-branch --depth 1 /usr/local/src/procps-${PROCPS_VERSION} && \
    cd /usr/local/src/procps-${PROCPS_VERSION} && \
    find . -type f -name '*.c' -print0 | xargs -0 -r -L1 perl -pi -e 's|/proc(?!ps)|/host/proc|g' && \
    ./autogen.sh && \
    ./configure --program-prefix=host && \
    make -j$(nproc) && \
    make install && \
    curl -fsSL https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-amd64.tar.gz | tar zx --no-same-owner -C /usr/local/bin crictl && \
    curl -fsSL https://github.com/duckdb/duckdb/releases/download/v1.1.3/duckdb_cli-linux-amd64.zip -o /tmp/duckdb_cli-linux-amd64.zip && \
    unzip /tmp/duckdb_cli-linux-amd64.zip -d /tmp && \
    mv /tmp/duckdb /usr/local/bin/duckdb && \
    rm /tmp/duckdb_cli-linux-amd64.zip && \
    curl -fsSL https://github.com/grafana/loki/releases/download/v2.4.2/logcli-linux-amd64.zip -o /tmp/logcli-linux-amd64.zip && \
    unzip /tmp/logcli-linux-amd64.zip -d /tmp && \
    mv /tmp/logcli-linux-amd64 /usr/local/bin/logcli && \
    rm /tmp/logcli-linux-amd64.zip && \
    curl -fsSL https://github.com/protocolbuffers/protobuf/releases/download/v29.1/protoc-29.1-linux-x86_64.zip -o /tmp/protoc-29.1-linux-x86_64.zip && \
    unzip /tmp/protoc-29.1-linux-x86_64.zip -d /usr/local && \
    rm /tmp/protoc-29.1-linux-x86_64.zip && \
    curl -fsSL https://toolbelt.treasuredata.com/sh/install-ubuntu-focal-td-agent4.sh | sh && \
    gem install fluent-plugin-stdin -v 0.1.2 && \
    gem install fluent-plugin-s3 -v 1.7.2 && \
    gem install fluent-plugin-gcs -v 0.4.2 && \
    curl -fsSL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o /tmp/awscliv2.zip && \
    unzip /tmp/awscliv2.zip -d /tmp && \
    /tmp/aws/install && \
    curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/google-cloud-cli.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/google-cloud-cli.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-cli.list && \
    curl -fsSL https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh | os=Ubuntu dist=jammy bash && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends google-cloud-cli speedtest && \
    pip install --no-cache-dir --break-system-packages cqlsh jupyter && \
    update-alternatives --install /usr/bin/python python /usr/bin/python3 1

RUN echo "admin:x:65532:65532::/home/admin:/usr/sbin/nologin" >> /etc/passwd
RUN echo "admin:x:65532:" >> /etc/group
RUN mkdir /home/admin && chown admin:admin /home/admin

RUN echo "admin:!:0:0:99999:7:::" >> /etc/shadow
RUN echo "%admin ALL=(ALL:ALL) NOPASSWD: ALL" | EDITOR="tee -a" visudo

RUN mkdir -p /var/duckdb
RUN <<EOS
cat <<EOSS > /var/duckdb/secret.sql
D CREATE SECRET minio (
    TYPE S3,
    KEY_ID 'minio',
    SECRET 'miniominio',
    ENDPOINT '127.0.0.1:9000',
    USE_SSL false,
    URL_STYLE 'path'
);
EOSS
EOS

COPY bin/armyknife /usr/local/bin/armyknife
COPY bin/insight /usr/local/bin/insight
COPY bin/l7sniff /usr/local/bin/l7sniff

USER 65532

WORKDIR /home/admin
