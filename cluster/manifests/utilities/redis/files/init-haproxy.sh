#!/usr/bin/env bash

set -e

HAPROXY_CONF=/usr/local/etc/haproxy/haproxy.cfg
cp /mnt/haproxy.cfg ${HAPROXY_CONF}

echo >> ${HAPROXY_CONF}
echo >> ${HAPROXY_CONF}

cat <<EOS >> ${HAPROXY_CONF}
frontend redis_master
  bind *:6380
  use_backend redis_master

backend redis_master
  mode tcp
  option tcp-check
  tcp-check connect
  tcp-check send PING\r\n
  tcp-check expect string +PONG
  tcp-check send info\ replication\r\n
  tcp-check expect string role:master
  tcp-check expect rstring connected_slaves:[$((QUORUM - 1))-9]([0-9]*)?
  tcp-check send QUIT\r\n
  tcp-check expect string +OK
EOS
for i in $(seq 0 $((REDIS_REPLICAS - 1))); do
  while ! getent hosts ${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local; do
    sleep 1
  done
  cat <<EOS >> ${HAPROXY_CONF}
  server R${i} ${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:6379 check inter 1s fall 1 rise 1 on-error mark-down resolvers kube-dns init-addr none
EOS
done

cat <<EOS >> ${HAPROXY_CONF}
frontend redis_slave
  bind *:6381
  use_backend redis_slave

backend redis_slave
  mode tcp
  option tcp-check
  tcp-check connect
  tcp-check send PING\r\n
  tcp-check expect string +PONG
  tcp-check send info\ replication\r\n
  tcp-check expect string role:slave
  tcp-check send QUIT\r\n
  tcp-check expect string +OK
EOS
for i in $(seq 0 $((REDIS_REPLICAS - 1))); do
  while ! getent hosts ${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local; do
    sleep 1
  done
  cat <<EOS >> ${HAPROXY_CONF}
  server R${i} ${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:6379 check inter 1s fall 1 rise 1 on-error mark-down resolvers kube-dns init-addr none
EOS
done
