#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly SENTINEL_CONFIGURATION=/etc/redis/sentinel.conf
readonly REDIS_PORT=6379
readonly SENTINEL_PORT=26379

mkdir -p "$(dirname "$SENTINEL_CONFIGURATION")"

cp /mnt/sentinel.conf "$SENTINEL_CONFIGURATION"

find() {
  master_ip=$(timeout 1 redis-cli -h "${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local" -p "$SENTINEL_PORT" sentinel get-master-addr-by-name mymaster | awk '/[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/' || true)
  if [ -n "$master_ip" ]; then
    if [ "$(timeout 1 redis-cli -h "$master_ip" ping)" == "PONG" ]; then
      sed -ri "2s/^/sentinel monitor mymaster $master_ip $REDIS_PORT $QUORUM \\n/" "$SENTINEL_CONFIGURATION"
    else
      echo "Cannot found healthy master" 1>&2
      sleep 1
      find
    fi
  else
    # Avoid "No such master with specified name" on index 0 as well
    # https://github.com/redis/redis/blob/5e3be1be09c947810732e7be2a4bb1b0ed75de4a/src/sentinel.c#L1871-L1874
    default_master="${SERVICE_NAME}-0.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local"
    if [ "$(timeout 1 redis-cli -h "$default_master" ping)" == "PONG" ]; then
      sed -ri "2s/^/sentinel monitor mymaster $default_master $REDIS_PORT $QUORUM \\n/" "$SENTINEL_CONFIGURATION"
    else
      echo "Cannot found healthy default master" 1>&2
      sleep 1
      find
    fi
  fi
}

find

announce_ip=$(getent hosts "${POD_NAME}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local" | cut -d" " -f1)
echo "sentinel announce-ip $announce_ip" >> "$SENTINEL_CONFIGURATION"
echo "sentinel announce-port $SENTINEL_PORT" >> "$SENTINEL_CONFIGURATION"

redis-sentinel "$SENTINEL_CONFIGURATION" --protected-mode no

restore() {
  cp /mnt/sentinel.conf "$SENTINEL_CONFIGURATION"
}

trap restore EXIT
