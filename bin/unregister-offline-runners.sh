#!/usr/bin/env bash

REPOSITORY="hippocampus-dev/hippocampus"

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

ids=()
page=1
while true; do
  result=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/actions/runners?per_page=100&page=${page}" | jq -re '.runners | map(select(.status == "offline")) | .[].id') || break
  ids+=(${result})
  page=$((page + 1))
done

for id in "${ids[@]}"; do
  curl -fsSL -X DELETE -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/actions/runners/${id}"
done
