#!/usr/bin/env bash
set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   e2e.sh TARGET_URL
EOS
}

args=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

url="${args[0]:-}"
if [ -z "$url" ]; then
  usage
  exit 1
fi

docker build -t chrome-devtools-protocol-server .
docker run -d --name chrome-devtools-protocol-server-e2e -p 59222:59222 -p 59223:59223 chrome-devtools-protocol-server
trap "docker rm -f chrome-devtools-protocol-server-e2e > /dev/null 2>&1 || true" EXIT
while ! curl -fsSLo /dev/null "http://127.0.0.1:59222/json/version" 2>/dev/null; do sleep 1; done
while ! curl -fsSLo /dev/null "http://127.0.0.1:59223/json/version" 2>/dev/null; do sleep 1; done
curl -fsSL -H "Host: chrome-devtools-protocol-server.example.com" "http://127.0.0.1:59223/json/version" | grep -q "ws://chrome-devtools-protocol-server.example.com/devtools/browser/"
cd playwright && npm ci && npm test -- "$url" && npm test -- --cdp http://127.0.0.1:59223 "$url"
