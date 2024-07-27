#!/usr/bin/env bash

set -eo pipefail

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

version=$1
if [ -z "$version" ]; then
  exit 1
fi

find $ENTRYPOINT/.. -type f -name Cargo.toml | while IFS= read -r f; do
  sed -ri "s/^version = \".*\"/version = \"$version\"/" $f
done

find $ENTRYPOINT/.. -type f -name pyproject.toml | while IFS= read -r f; do
  sed -ri "s/^version = \".*\"/version = \"$version\"/" $f
done

bash $ENTRYPOINT/poetry-update.sh --no-update

find $ENTRYPOINT/.. -type f -name version.go | while IFS= read -r f; do
  sed -ri "s/^const Version = \".*\"/const Version = \"$version\"/" $f
done

find $ENTRYPOINT/.. -type f -name tauri.conf.json | while IFS= read -r f; do
  sed -ri "s/\"version\": \".*\"/\"version\": \"$version\"/" $f
done

sed -ri  "s/flag.StringVar\(&binaryVersion, \"binary-version\", \".*\",/flag.StringVar\(&binaryVersion, \"binary-version\", \"$version\",/" $ENTRYPOINT/../cluster/applications/github-actions-runner-controller/main.go
