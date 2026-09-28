#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   strip-slide-image-metadata.sh <directory>...

Strip the metadata off every JPEG and GIF under each directory in place, so a
built deck carries none of it. Symlinks are resolved and their targets rewritten.
EOS
}

args=()
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

if [ "${#args[@]}" -eq 0 ]; then
  usage
  exit 1
fi

function rewrite_in_place() {
  local target="$1"
  shift

  if "$@" && chmod --reference="$target" "${target}.tmp" && mv "${target}.tmp" "$target"; then
    return
  fi

  rm -f "${target}.tmp"
  return 1
}

for directory in "${args[@]}"; do
  if [ ! -d "$directory" ]; then
    echo "No such directory $directory" 1>&2
    exit 1
  fi

  while IFS= read -r image; do
    target=$(readlink -f "$image")
    rewrite_in_place "$target" jpegtran -copy none -optimize -outfile "${target}.tmp" "$target"
  done < <(find -L "$directory" -type f \( -iname '*.jpg' -o -iname '*.jpeg' \))

  # --no-extensions covers the application extensions --no-comments and --no-names leave behind; the loop count lives in its own field, not in the extension list, so an animation still loops afterwards.
  while IFS= read -r image; do
    target=$(readlink -f "$image")
    rewrite_in_place "$target" gifsicle --no-comments --no-names --no-extensions -O2 -o "${target}.tmp" "$target"
  done < <(find -L "$directory" -type f -iname '*.gif')
done
