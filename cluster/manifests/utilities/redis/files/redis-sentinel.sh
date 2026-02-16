#!/usr/bin/env bash

set -e

SENTINEL_CONF=/etc/redis/sentinel.conf
REDIS_PORT=6379
SENTINEL_PORT=26379

mkdir -p $(dirname ${SENTINEL_CONF})

cp /mnt/sentinel.conf ${SENTINEL_CONF}

function find() {
  master_ip=$(timeout 1 redis-cli -h ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local -p ${SENTINEL_PORT} sentinel get-master-addr-by-name mymaster | grep -E "[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}" || true)
  if [ "${master_ip}" ]; then
    if [ "$(timeout 1 redis-cli -h "${master_ip}" ping)" == "PONG" ]; then
      sed -ri "2s/^/sentinel monitor mymaster ${master_ip} ${REDIS_PORT} ${QUORUM} \\n/" ${SENTINEL_CONF}
    else
      echo "Cannot found healthy master" 1>&2
      sleep 1
      find
    fi
  else
    if [ "${INDEX}" != "0" ]; then
      default_master="${SERVICE_NAME}-0.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local"
      if [ "$(timeout 1 redis-cli -h "${default_master}" ping)" == "PONG" ]; then
        sed -ri "2s/^/sentinel monitor mymaster ${default_master} ${REDIS_PORT} ${QUORUM} \\n/" ${SENTINEL_CONF}
      else
        echo "Cannot found healthy default master" 1>&2
        sleep 1
        find
      fi
    fi
  fi
}

find

ANNOUNCE_IP=$(getent hosts "${POD_NAME}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local" | cut -d" " -f1)
echo "sentinel announce-ip ${ANNOUNCE_IP}" >> ${SENTINEL_CONF}
echo "sentinel announce-port ${SENTINEL_PORT}" >> ${SENTINEL_CONF}

redis-sentinel ${SENTINEL_CONF} --protected-mode no

function restore() {
  cp /mnt/sentinel.conf ${SENTINEL_CONF}
}

trap restore EXIT
