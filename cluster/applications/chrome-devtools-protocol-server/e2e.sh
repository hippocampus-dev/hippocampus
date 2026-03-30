#!/usr/bin/env bash
set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

docker build -t chrome-devtools-protocol-server .
docker run -d --name chrome-devtools-protocol-server-e2e -p 59222:59222 chrome-devtools-protocol-server
trap "docker rm -f chrome-devtools-protocol-server-e2e > /dev/null 2>&1 || true" EXIT
while ! curl -fsSLo /dev/null "http://127.0.0.1:59222/json/version" 2>/dev/null; do sleep 1; done
cd playwright && npm ci && npm test -- "$1"
