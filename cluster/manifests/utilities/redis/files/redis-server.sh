#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly INDEX="${POD_NAME##*-}"
readonly REDIS_CONFIGURATION=/etc/redis/redis.conf
readonly REDIS_PORT=6379
readonly SENTINEL_PORT=26379

mkdir -p "$(dirname "$REDIS_CONFIGURATION")"

cp /mnt/redis.conf "$REDIS_CONFIGURATION"

echo "maxmemory $MEMORY_REQUESTS" >> "$REDIS_CONFIGURATION"

find() {
  master_ip=$(timeout 1 redis-cli -h "${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local" -p "$SENTINEL_PORT" sentinel get-master-addr-by-name mymaster | awk '/[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/' || true)
  if [ -n "$master_ip" ]; then
    if [ "$(timeout 1 redis-cli -h "$master_ip" ping)" == "PONG" ]; then
      echo "replicaof $master_ip $REDIS_PORT" >> "$REDIS_CONFIGURATION"
    else
      echo "Cannot found healthy master" 1>&2
      sleep 1
      find
    fi
  else
    if [ "$INDEX" != "0" ]; then
      default_master="${SERVICE_NAME}-0.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local"
      if [ "$(timeout 1 redis-cli -h "$default_master" ping)" == "PONG" ]; then
        echo "replicaof $default_master $REDIS_PORT" >> "$REDIS_CONFIGURATION"
      else
        echo "Cannot found healthy default master" 1>&2
        sleep 1
        find
      fi
    fi
  fi
}

find

announce_ip=$(getent hosts "${POD_NAME}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local" | cut -d" " -f1)
echo "replica-announce-ip $announce_ip" >> "$REDIS_CONFIGURATION"
echo "replica-announce-port $REDIS_PORT" >> "$REDIS_CONFIGURATION"

redis-server "$REDIS_CONFIGURATION" --protected-mode no

restore() {
  cp /mnt/redis.conf "$REDIS_CONFIGURATION"
}

trap restore EXIT
