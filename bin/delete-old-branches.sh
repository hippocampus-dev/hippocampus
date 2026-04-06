#!/usr/bin/env bash

REPOSITORY="hippocampus-dev/hippocampus"
CUTOFF_DAYS=30

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

branches=()
page=1
while true; do
  result=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/branches?per_page=100&page=${page}" | jq -re '.[].name') || break
  branches+=(${result})
  page=$((page + 1))
done

for branch in "${branches[@]}"; do
  if [ "$branch" = "main" ]; then
    continue
  fi
  timestamp=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/commits/${branch}" | jq -re '.commit.committer.date | fromdateiso8601')
  if [ "$timestamp" -lt "$(date -d "$CUTOFF_DAYS days ago" +%s)" ]; then
    curl -fsSL -X DELETE -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $GITHUB_TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/git/refs/heads/${branch}"
  fi
done
