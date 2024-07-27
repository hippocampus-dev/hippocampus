# syntax=docker/dockerfile:1.4

FROM ubuntu:22.04

ARG DEBIAN_FRONTEND=noninteractive

ENV LOKI_ADDR http://loki-query-frontend.loki.svc.cluster.local:3100
ENV MIMIR_ADDRESS http://127.0.0.1:3100
ENV MIMIR_TENANT_ID anonymous
ENV MEMCACHED_SERVERS 127.0.0.1:11211

ENV PROCPS_VERSION v4.0.4
ENV CRICTL_VERSION v1.26.0

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git automake ruby ruby-dev nodejs npm python3 python3-pip sudo gcc make autoconf libtool gettext autopoint pkg-config libncurses5-dev unzip apt-transport-https ca-certificates gnupg vim tcpdump tshark netcat net-tools curl nghttp2 jq dnsutils iputils-ping iproute2 redis-tools libmemcached-tools mysql-client postgresql-client bc

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked --mount=type=cache,target=/usr/local/src \
    [ -d /usr/local/src/procps-${PROCPS_VERSION} ] || git clone https://gitlab.com/procps-ng/procps.git -b ${PROCPS_VERSION} --single-branch --depth=1 /usr/local/src/procps-${PROCPS_VERSION} && \
    cd /usr/local/src/procps-${PROCPS_VERSION} && \
    find . -type f -name '*.c' | xargs -L1 perl -pi -e 's|/proc(?!ps)|/host/proc|g' && \
    ./autogen.sh && \
    ./configure --program-prefix=host && \
    make -j$(nproc) && \
    make install && \
    curl -fsSL https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-amd64.tar.gz | tar zx --no-same-owner -C /usr/local/bin crictl && \
    curl -fsSL https://dl.min.io/client/mc/release/linux-amd64/mc -o /usr/local/bin/mc && \
    chmod +x /usr/local/bin/mc && \
    curl -fsSL https://github.com/grafana/loki/releases/download/v2.4.2/logcli-linux-amd64.zip -o /tmp/logcli-linux-amd64.zip && \
    unzip /tmp/logcli-linux-amd64.zip -d /tmp && \
    mv /tmp/logcli-linux-amd64 /usr/local/bin/logcli && \
    rm /tmp/logcli-linux-amd64.zip && \
    curl -fsSL https://toolbelt.treasuredata.com/sh/install-ubuntu-focal-td-agent4.sh | sh && \
    gem install fluent-plugin-stdin -v 0.1.2 && \
    gem install fluent-plugin-s3 -v 1.7.2 && \
    gem install fluent-plugin-gcs -v 0.4.2 && \
    curl -fsSL https://github.com/grafana/mimir/releases/download/mimir-2.11.0/mimirtool-linux-amd64 -o /usr/local/bin/mimirtool && \
    chmod +x /usr/local/bin/mimirtool && \
    curl -fsSL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o /tmp/awscliv2.zip && \
    unzip /tmp/awscliv2.zip -d /tmp && \
    /tmp/aws/install && \
    pip install --no-cache-dir cqlsh && \
    curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/google-cloud-cli.gpg add - && \
    echo "deb [signed-by=/usr/share/keyrings/google-cloud-cli.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-cli.list && \
    curl -fsSL https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh | bash && \
    apt-get update -y && \
    apt-get -y install google-cloud-cli && \
    apt-get install -y --no-install-recommends speedtest

RUN echo "admin:x:65532:65532::/home/admin:/usr/sbin/nologin" >> /etc/passwd
RUN echo "admin:x:65532:" >> /etc/group
RUN mkdir /home/admin && chown admin:admin /home/admin

RUN echo "admin:!:0:0:99999:7:::" >> /etc/shadow
RUN echo "%admin ALL=(ALL:ALL) NOPASSWD: ALL" | EDITOR="tee -a" visudo

COPY tmp/armyknife /usr/local/bin/armyknife
COPY tmp/insight /usr/local/bin/insight
COPY tmp/l7sniff /usr/local/bin/l7sniff

USER 65532

WORKDIR /home/admin
