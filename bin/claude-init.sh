#!/usr/bin/env bash

set -e

function usage() {
  cat <<EOS
Usage:
   claude-init.sh [--force] [--interval <seconds>]
EOS
}

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

args=()
flags=()
FORCE=0
INTERVAL=0
while (( $# )); do
  case "${1}" in
    -h|--help)
      usage
      exit 0
      ;;
    --force|-f)
      FORCE=1
      shift
      ;;
    --interval)
      INTERVAL=${2}
      shift 2
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag ${1}" 1>&2
      exit 1
      ;;
    *)
      args+=("${1}")
      shift
      ;;
  esac
done

find ${ENTRYPOINT}/../{packages,cluster/applications,cluster/applications/packages,cluster/manifests} -maxdepth 1 -mindepth 1 -type d | while IFS= read -r d; do
  if [ ! -f ${d}/CLAUDE.md ] || [ ${FORCE} -eq 1 ]; then
    echo "---------------------------------------------"
    echo "Initializing $(realpath $d)"
    echo "---------------------------------------------"

    (
      cd ${d}
      claude /init --allowedTools "Task,Glob,Grep,LS,Read,Edit,MultiEdit,Write,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*)" --print < /dev/null
    )

    sleep $INTERVAL
  fi
  (
    cd ${d}
    ln -sf CLAUDE.md AGENTS.md
  )
done
