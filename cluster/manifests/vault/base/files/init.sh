#!/usr/bin/env sh

set -e

readonly UNSEAL_KEYS_FILE=/data/unsealKeys
readonly ROOT_TOKEN_FILE=/data/rootToken

until vault status -tls-skip-verify > /dev/null 2>&1 || [ $? -eq 2 ]; do sleep 1; done

if vault status -tls-skip-verify | grep -E 'Initialized\s+false' > /dev/null 2>&1; then
  output=$(vault operator init)

  echo "$output" | awk '/Unseal Key/ {print $NF}' > "$UNSEAL_KEYS_FILE"
  echo "$output" | awk '/Initial Root Token/ {print $NF}' > "$ROOT_TOKEN_FILE"
fi

if vault status -tls-skip-verify | grep -E 'Sealed\s+true' > /dev/null 2>&1; then
  threshold=$(vault status -tls-skip-verify | awk '/Threshold/ {print $NF}')

  for i in $(seq 1 "$threshold"); do
    vault operator unseal "$(awk "NR==${i}" "$UNSEAL_KEYS_FILE")"
  done
fi
