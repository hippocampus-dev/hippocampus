#!/usr/bin/env bash

set -e

JB_HOME=${JB_HOME:-../jsonnet-bundler}

hash=$(cat jsonnetfile.lock.json | sha256sum | cut -d ' ' -f 1)
if [ ! -f "${JB_HOME}/.jb-hash" ] || [ "$(cat "${JB_HOME}/.jb-hash")" != "$hash" ]; then
  jb install --jsonnetpkg-home="$JB_HOME"
  echo "$hash" > "${JB_HOME}/.jb-hash"
fi

if [ "$1" == "--only-install" ]; then
  exit 0
fi
jsonnet -J "$JB_HOME" -J . "$@"
