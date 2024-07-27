#!/usr/bin/env bash

set -eo pipefail

MCROUTER_CONFIG=/etc/mcrouter/config.json

cat <<EOS >> $MCROUTER_CONFIG
{
  "pools": {
    "A": {
      "servers": [
EOS

for i in $(seq 0 $((MEMCACHED_REPLICAS - 1))); do
  while ! getent hosts ${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local; do
    sleep 1
  done
  cat <<EOS >> $MCROUTER_CONFIG
        "${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:11211",
EOS
done

cat <<EOS >> $MCROUTER_CONFIG
      ]
    }
  },
  "route": {
    "type": "OperationSelectorRoute",
    "default_policy": "PoolRoute|A",
    "operation_policies": {
      "get": "LatestRoute|Pool|A",
      "add": "AllSyncRoute|Pool|A",
      "delete": "AllSyncRoute|Pool|A",
      "set": "AllSyncRoute|Pool|A"
    }
  }
}
EOS
