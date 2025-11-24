#!/usr/bin/env bash

REPOSITORY="kaidotio/hippocampus"
CUTOFF_DAYS=30

set -e

curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/branches?per_page=100" | jq -r '.[].name' | grep -v main | while read -r branch; do
  timetamp=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/commits/${branch}" | jq -r '.commit.committer.date | fromdateiso8601')
  if [[ "${timetamp}" < "$(date -d "${CUTOFF_DAYS} days ago" +%s)" ]]; then
    curl -fsSL -X DELETE -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/git/refs/heads/${branch}"
  fi
done
