#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

usage() {
  cat <<EOS
Usage:
   delete-old-branches.sh
EOS
}

readonly REPOSITORY="hippocampus-dev/hippocampus"
readonly CUTOFF_DAYS=30

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

branches=()
page=1
while true; do
  result=$(curl -fsSL --retry 5 --retry-all-errors -H "Accept: application/vnd.github+json" -H @<(printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN") -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/branches?per_page=100&page=${page}" | jq -re '.[].name') || break
  branches+=(${result})
  page=$((page + 1))
done

for branch in "${branches[@]}"; do
  if [ "$branch" = "main" ]; then
    continue
  fi
  timestamp=$(curl -fsSL --retry 5 --retry-all-errors -H "Accept: application/vnd.github+json" -H @<(printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN") -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/commits/${branch}" | jq -re '.commit.committer.date | fromdateiso8601')
  if [ "$timestamp" -lt "$(date -d "$CUTOFF_DAYS days ago" +%s)" ]; then
    curl -fsSL --retry 5 --retry-all-errors -X DELETE -H "Accept: application/vnd.github+json" -H @<(printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN") -H "X-GitHub-Api-Version: 2022-11-28" "https://api.github.com/repos/${REPOSITORY}/git/refs/heads/${branch}"
  fi
done
