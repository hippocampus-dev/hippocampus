#!/usr/bin/env bash

INDEX=${POD_NAME##*-}
PORT=6335
FIRST_PEER="http://${SERVICE_NAME}-0.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:${PORT}"

option="--uri ${FIRST_PEER}"
if [ "${INDEX}" != "0" ]; then
  option="--bootstrap ${FIRST_PEER} --uri http://${SERVICE_NAME}-${INDEX}.${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:${PORT}"
fi

# Script to run qdrant in docker container and handle contingencies, like OOM.
# The functioning logic is as follows:
# - If recovery mode is allowed, we check if qdrant was killed during initialization or not.
#   - If it was killed during initialization, we remove run qdrant in recovery mode
#   - If it was killed after initialization, do nothing and restart container
# - If recovery mode is not allowed, we just restart container

/qdrant/qdrant ${@} ${option}

EXIT_CODE=$?

QDRANT_ALLOW_RECOVERY_MODE=${QDRANT_ALLOW_RECOVERY_MODE:-false}

# Check that recovery mode is allowed
if [ "${QDRANT_ALLOW_RECOVERY_MODE}" != "true" ]; then
    exit ${EXIT_CODE}
fi

# Check that qdrant was killed (exit code 137)
# Ideally, we want to catch only OOM, but it's not possible to distinguish it from random kill signal
if [ ${EXIT_CODE} -ne 137 ]; then
    exit ${EXIT_CODE}
fi

IS_INITIALIZED_FILE=".qdrant-initialized"
RECOVERY_MESSAGE="Qdrant was killed during initialization. Most likely it's Out-of-Memory.
Please check memory consumption, increase memory limit or remove some collections and restart"

# Check that qdrant was initialized
# Qdrant creates IS_INITIALIZED_FILE file after initialization
# So if it doesn't exist, qdrant was killed during initialization
if [ ! -f "${IS_INITIALIZED_FILE}" ]; then
    # Run qdrant in recovery mode.
    # No collection operations are allowed in recovery mode except for removing collections
    QDRANT__STORAGE__RECOVERY_MODE="${RECOVERY_MESSAGE}" /qdrant/qdrant ${@} ${option}
    exit $?
fi

exit ${EXIT_CODE}
