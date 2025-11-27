#!/usr/bin/env bash

REPOSITORY="kaidotio/hippocampus"

set -e

curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/runners | jq '.runners | map(select(.status == "offline")) | .[].id' | xargs -r -I{} curl -fsSL -X DELETE -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "X-GitHub-Api-Version: 2022-11-28" https://api.github.com/repos/${REPOSITORY}/actions/runners/{}
