#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   inline-slide-images.sh <html>...

Rewrite the images/ references marp-cli emits into data: URIs so that a built
deck is distributable as a single self-contained HTML file.
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

for html in "${args[@]}"; do
  directory=$(dirname "$html")
  [ -d "${directory}/images" ] || continue

  content=$(cat "$html")
  # marp emits the same reference both as <img src="images/x.jpg"> and as
  # <figure style="background-image:url(&quot;images/x.jpg&quot;)">, so replacing the
  # literal path covers both without parsing the HTML.
  while IFS= read -r image; do
    reference="${image#"${directory}/"}"
    mime_type=$(file -bL --mime-type "$image")
    content="${content//"${reference}"/data:${mime_type};base64,$(base64 -w0 "$image")}"
  done < <(find -L "${directory}/images" -type f)

  # markdown-it percent-encodes non-ASCII paths, so those never match what find
  # returned. Fail instead of writing an HTML that silently still needs images/.
  case "$content" in
    *'src="images/'*|*'url("images/'*|*'url(&quot;images/'*)
      echo "Unresolved image reference in $html" 1>&2
      exit 1
      ;;
  esac

  # marp leaves no trailing newline, and $(cat) already stripped any.
  printf '%s' "$content" > "$html"
  echo "$html"
done
