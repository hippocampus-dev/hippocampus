#!/usr/bin/env bash

REPOSITORY="hippocampus-dev/hippocampus"
CUTOFF_DAYS=30

set -eo pipefail

branches=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/branches?per_page=100" | jq -re '.[].name' | awk '!/^main$/')

for branch in ${branches}; do
  timestamp=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/commits/${branch}" | jq -re '.commit.committer.date | fromdateiso8601')
  if [ "${timestamp}" -lt "$(date -d "${CUTOFF_DAYS} days ago" +%s)" ]; then
    curl -fsSL -X DELETE -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/git/refs/heads/${branch}"
  fi
done
