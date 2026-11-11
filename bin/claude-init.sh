#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

usage() {
  cat <<EOS
Usage:
   claude-init.sh [--force] [--interval SECONDS]
EOS
}

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
readonly ENTRYPOINT

args=()
flags=()
force=0
interval=0
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    -f|--force)
      force=1
      shift
      ;;
    --interval)
      interval="$2"
      shift 2
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

while IFS= read -r d; do
  if [ ! -f "${d}/CLAUDE.md" ] || [ "$force" -eq 1 ]; then
    echo "---------------------------------------------"
    echo "Initializing $(realpath "$d")"
    echo "---------------------------------------------"

    (
      cd "$d"
      claude /init --allowedTools "Task,Glob,Grep,LS,Read,Edit,MultiEdit,Write,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*)" --print < /dev/null
    )

    sleep "$interval"
  fi
  if [ -f "${d}/CLAUDE.md" ]; then
    (
      cd "$d"
      ln -sf CLAUDE.md AGENTS.md
    )
  fi
done < <(find -L "${ENTRYPOINT}/../"{packages,cluster/applications,cluster/applications/packages,cluster/manifests} -maxdepth 1 -mindepth 1 -type d)
