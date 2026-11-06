#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

usage() {
  cat <<EOS
Usage:
   unregister-offline-runners.sh
EOS
}

readonly REPOSITORY="hippocampus-dev/hippocampus"

args=()
flags=()
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

ids=()
page=1
while true; do
  result=$(curl -fsSL --retry 5 --retry-all-errors -H "Accept: application/vnd.github+json" -H @<(printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN") -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/actions/runners?per_page=100&page=${page}" | jq -re '.runners | map(select(.status == "offline")) | .[].id') || break
  ids+=(${result})
  page=$((page + 1))
done

for id in "${ids[@]}"; do
  curl -fsSL --retry 5 --retry-all-errors -X DELETE -H "Accept: application/vnd.github+json" -H @<(printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN") -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/actions/runners/${id}"
done
