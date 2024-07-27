#!/usr/bin/env sh

set -e

UNSEAL_KEYS_FILE=/data/unsealKeys
ROOT_TOKEN_FILE=/data/rootToken

sleep 10

if vault status -tls-skip-verify | grep -E 'Initialized\s+false' > /dev/null 2>&1; then
  t=$(mktemp)

  vault operator init > $t

  cat $t | grep 'Unseal Key' | awk '{print $NF}' > $UNSEAL_KEYS_FILE
  cat $t | grep 'Initial Root Token' | awk '{print $NF}' > $ROOT_TOKEN_FILE
fi

if vault status -tls-skip-verify | grep -E 'Sealed\s+true' > /dev/null 2>&1; then
  threshold=$(vault status -tls-skip-verify | grep 'Threshold' | awk '{print $NF}')
  unsealKeys=$(cat $UNSEAL_KEYS_FILE)

  for i in $(seq 1 ${threshold}); do
    vault operator unseal $(echo $unsealKeys | cut -d' ' -f $i)
  done
fi
