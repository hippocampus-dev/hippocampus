#!/usr/bin/env bash

set -eo pipefail

function usage() {
  cat <<EOS
Usage:
   bump.sh VERSION
EOS
}

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")"; pwd)

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

version="${args[0]:-}"
if [ -z "$version" ]; then
  usage
  exit 1
fi

while IFS= read -r f; do
  sed -ri "s/^version = \".*\"/version = \"${version}\"/" "$f"
done < <(find "${ENTRYPOINT}/.." -type f -name Cargo.toml)

while IFS= read -r f; do
  sed -ri "s/^version = \".*\"/version = \"${version}\"/" "$f"
done < <(find "${ENTRYPOINT}/.." -type f -name pyproject.toml)

bash "${ENTRYPOINT}/poetry-update.sh" --no-update

while IFS= read -r f; do
  sed -ri "s/^const Version = \".*\"/const Version = \"${version}\"/" "$f"
done < <(find "${ENTRYPOINT}/.." -type f -name version.go)

while IFS= read -r f; do
  sed -ri "s/\"version\": \".*\"/\"version\": \"${version}\"/" "$f"
done < <(find "${ENTRYPOINT}/.." -type f -name tauri.conf.json)

sed -ri  "s/flag.StringVar\(&binaryVersion, \"binary-version\", \".*\",/flag.StringVar\(&binaryVersion, \"binary-version\", \"${version}\",/" "${ENTRYPOINT}/../cluster/applications/github-actions-runner-controller/main.go"
