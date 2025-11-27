#!/usr/bin/env bash

set -e

MCROUTER_CONFIG=/etc/mcrouter/config.json

cat <<EOS >> ${MCROUTER_CONFIG}
{
  "pools": {
    "A": {
      "servers": [
EOS

for i in $(seq 0 $((MEMCACHED_REPLICAS - 1))); do
  while ! getent hosts ${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local; do
    sleep 1
  done
  cat <<EOS >> ${MCROUTER_CONFIG}
        "${SERVICE_NAME}-${i}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:11211",
EOS
done

OPERATION_POLICY=${OPERATION_POLICY:-"sync"}

if [ "${OPERATION_POLICY}" == "sync" ]; then
  cat <<EOS >> ${MCROUTER_CONFIG}
      ]
    }
  },
  "route": {
    "type": "OperationSelectorRoute",
    "default_policy": "AllSyncRoute|Pool|A",
    "operation_policies": {
      "get": "LatestRoute|Pool|A",
      "gets": "LatestRoute|Pool|A"
    }
  }
}
EOS
  exit 0
fi

if [ "${OPERATION_POLICY}" == "async" ]; then
  cat <<EOS >> ${MCROUTER_CONFIG}
      ]
    }
  },
  "route": {
    "type": "OperationSelectorRoute",
    "default_policy": "AllAsyncRoute|Pool|A",
    "operation_policies": {
      "get": "LatestRoute|Pool|A",
      "gets": "LatestRoute|Pool|A"
    }
  }
}
EOS
  exit 0
fi

if [ "${OPERATION_POLICY}" == "hash" ]; then
  cat <<EOS >> ${MCROUTER_CONFIG}
      ]
    }
  },
  "route": "HashRoute|Pool|A"
}
EOS
  exit 0
fi

echo "Invalid operation policy: ${OPERATION_POLICY}" 1>&2
exit 1
