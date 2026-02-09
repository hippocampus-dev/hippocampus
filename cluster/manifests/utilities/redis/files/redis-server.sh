#!/usr/bin/env bash

set -e

INDEX=${POD_NAME##*-}
REDIS_CONF=/etc/redis/redis.conf
REDIS_PORT=6379
SENTINEL_PORT=26379

mkdir -p $(dirname ${REDIS_CONF})

cp /mnt/redis.conf ${REDIS_CONF}

echo "maxmemory ${MEMORY_REQUESTS}" >> ${REDIS_CONF}

function find() {
  master_ip=$(timeout 1 redis-cli -h ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local -p ${SENTINEL_PORT} sentinel get-master-addr-by-name mymaster | grep -E '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}' || true)
  if [ "${master_ip}" ]; then
    if [ "$(timeout 1 redis-cli -h ${master_ip} ping)" == "PONG" ]; then
      echo "replicaof ${master_ip} ${REDIS_PORT}" >> ${REDIS_CONF}
    else
      echo "Cannot found healthy master" 1>&2
      sleep 1
      find
    fi
  else
    if [ "${INDEX}" != "0" ]; then
      default_master="${SERVICE_NAME}-0.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local"
      if [ "$(timeout 1 redis-cli -h ${default_master} ping)" == "PONG" ]; then
        echo "replicaof ${default_master} ${REDIS_PORT}" >> ${REDIS_CONF}
      else
        echo "Cannot found healthy default master" 1>&2
        sleep 1
        find
      fi
    fi
  fi
}

find

ANNOUNCE_IP=$(getent hosts ${POD_NAME}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local | cut -d' ' -f1)
echo "replica-announce-ip ${ANNOUNCE_IP}" >> ${REDIS_CONF}
echo "replica-announce-port ${REDIS_PORT}" >> ${REDIS_CONF}

redis-server ${REDIS_CONF} --protected-mode no

function restore() {
  cp /mnt/redis.conf ${REDIS_CONF}
}

trap restore EXIT
