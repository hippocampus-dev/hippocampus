#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

usage() {
  cat <<EOS
Usage:
   decrypt.sh
EOS
}

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
readonly ENTRYPOINT

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

while IFS= read -r file; do
  armyknife rails credentials:show "$file" > "${file%.enc}"
done < <(find -L "${ENTRYPOINT}/.." -type f -name "*.enc")
