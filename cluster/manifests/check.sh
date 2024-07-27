#!/usr/bin/env bash

set -eo pipefail

find . -maxdepth 1 -mindepth 1 -type d -not -name utilities | while IFS= read -r manifest; do
  cp ${manifest}/overlays/dev/kustomization.yaml ${manifest}/overlays/dev/kustomization.yaml.bak
  cat ${manifest}/overlays/dev/kustomization.yaml.bak | ruby -r yaml -r json -e 'puts JSON.generate(YAML.load(STDIN.read))'
  k=$(kustomize build ${manifest}/overlays/dev --enable-alpha-plugins)

  if echo "$k" | grep -q "kind: Namespace"; then
    if ! echo "$k" | grep -q "kind: NetworkPolicy"; then
      echo "Namespace found but NetworkPolicy not found in $manifest"
    fi
  fi
done
