#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

usage() {
  cat <<EOS
Usage:
   colabofy.sh [options] -- COMMAND...

Options:
   --gpu GPU          Accelerator to allocate: T4, L4, G4, H100 or A100 (default: T4)
   --port PORT        Forward 127.0.0.1:PORT to PORT on the runtime while COMMAND runs
   --input PATH       Link PATH under the home directory to the same place under the runtime's home through ${CACHE}, which drops the least recently used files beyond 100GiB (repeatable)
   --output PATH      Copy PATH back into the current directory after COMMAND exits (repeatable)
   --timeout SECONDS  Longest silence from COMMAND that colab exec tolerates (default: 86400)
   -h, --help         Show this help
EOS
}

readonly CACHE="gdrive:colabofy"
readonly CACHE_LIMIT=$((100 * 1024 ** 3))

gpu=T4
port=""
inputs=()
outputs=()
timeout=86400
args=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --gpu)
      gpu="$2"
      shift 2
      ;;
    --port)
      port="$2"
      shift 2
      ;;
    --input)
      inputs+=("$2")
      shift 2
      ;;
    --output)
      outputs+=("$2")
      shift 2
      ;;
    --timeout)
      timeout="$2"
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

if [ "${#args[@]}" -ne 0 ] || [ "$#" -eq 0 ]; then
  usage
  exit 1
fi

session="colabofy-${RANDOM}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

while IFS= read -r -d "" file; do
  # .env points at endpoints on this machine (HTTP_PROXY=http://127.0.0.1:18080 in cluster/applications/meegeren/.env)
  [ -f "$file" ] && [ "${file##*/}" != ".env" ] || continue
  printf '%s\0' "$file"
done < <(git ls-files -z --cached --others --exclude-standard) | tar --null -T - -czf "${work}/archive.tar.gz"

files=""
if [ "${#inputs[@]}" -ne 0 ]; then
  files=$(find -L "${inputs[@]}" -type f -print0 | xargs -r -0 realpath -s --relative-to="$HOME" | LC_ALL=C sort -u)
  if [ -z "$files" ]; then
    echo "--input lists no file" 1>&2
    exit 1
  fi
  if [[ $'\n'"$files" =~ $'\n'\.\./ ]]; then
    echo "--input must be under ${HOME}" 1>&2
    exit 1
  fi

  state="${XDG_STATE_HOME:-${HOME}/.local/state}/colabofy/state.json"
  mkdir -p "$(dirname "$state")"
  [ -f "$state" ] || echo "{}" > "$state"
  exec {lock}> "${XDG_RUNTIME_DIR:-/tmp}/colabofy.lock"
  flock "$lock"

  rclone mkdir "$CACHE"
  remote=$(rclone lsjson -R --files-only "$CACHE")
  size=$(cd ~ && xargs -r -d '\n' stat -L -c %s <<< "$files" | awk '{ total += $1 } END { print total + 0 }')
  evicted=$(jq -rn --slurpfile remote <(printf '%s' "$remote") --slurpfile state "$state" --rawfile current <(printf '%s\n' "$files") --argjson size "$size" --argjson limit "$CACHE_LIMIT" '
    ($current | split("\n")) as $current
    | [$remote[0][] | select(.Path | IN($current[]) | not)]
    | sort_by($state[0][.Path] // 0)
    | reduce .[] as $file ({total: ($size + (map(.Size) | add // 0)), evicted: []};
        if .total > $limit then {total: (.total - $file.Size), evicted: (.evicted + [$file.Path])} else . end)
    | .evicted[]')
  if [ -n "$evicted" ]; then
    rclone delete --drive-use-trash=false --files-from-raw <(printf '%s\n' "$evicted") "$CACHE"
  fi
  rclone copy --copy-links --files-from-raw <(printf '%s\n' "$files") ~ "$CACHE"
  jq --rawfile current <(printf '%s\n' "$files") --rawfile evicted <(printf '%s\n' "$evicted") --argjson now "$(date +%s)" '
    delpaths($evicted | split("\n") | map(select(. != "") | [.]))
    + ($current | split("\n") | map(select(. != "") | {(.): $now}) | add)' "$state" > "${state}.tmp"
  mv "${state}.tmp" "$state"
  exec {lock}>&-
fi

archive=$(base64 -w 0 "${work}/archive.tar.gz")
encoded_command=$(jq -cn '$ARGS.positional' --args -- "$@" | base64 -w 0)
encoded_inputs=$(jq -cRn '[inputs | select(. != "")]' <<< "$files" | base64 -w 0)
encoded_outputs=$(jq -cn '$ARGS.positional' --args -- "${outputs[@]}" | base64 -w 0)
encoded_project=$(printf '%s' "${PWD##*/}" | base64 -w 0)
{
  printf 'ARCHIVE = "%s"\n' "$archive"
  printf 'COMMAND = "%s"\n' "$encoded_command"
  printf 'INPUTS = "%s"\n' "$encoded_inputs"
  printf 'OUTPUTS = "%s"\n' "$encoded_outputs"
  printf 'PROJECT = "%s"\n' "$encoded_project"
  cat <<EOS

import base64
import io
import json
import os
import shutil
import subprocess
import sys
import tarfile

directory = os.path.join("/content", base64.b64decode(PROJECT).decode())
status = 1
try:
    with tarfile.open(fileobj=io.BytesIO(base64.b64decode(ARCHIVE))) as archive:
        archive.extractall(directory)
    for path in json.loads(base64.b64decode(INPUTS)):
        destination = os.path.join(os.path.expanduser("~"), path)
        os.makedirs(os.path.dirname(destination), exist_ok=True)
        os.symlink(os.path.join("/content/drive/MyDrive/colabofy", path), destination)

    environment = dict(os.environ, UV_MANAGED_PYTHON="1", PYTHONUNBUFFERED="1")
    if shutil.which("uv") is None:
        subprocess.run([sys.executable, "-m", "pip", "install", "--quiet", "uv"], check=True)
        import uv

        environment["PATH"] = os.path.dirname(uv.find_uv_bin()) + os.pathsep + environment["PATH"]

    process = subprocess.Popen(
        json.loads(base64.b64decode(COMMAND)),
        cwd=directory,
        env=environment,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    for line in process.stdout:
        print(line, end="", flush=True)
    status = process.wait()
    if status < 0:
        status = 128 - status
finally:
    os.makedirs("/content/.colabofy")
    with tarfile.open("/content/.colabofy/output.tar.gz", "w:gz") as archive:
        for output in json.loads(base64.b64decode(OUTPUTS)):
            if os.path.exists(os.path.join(directory, output)):
                archive.add(os.path.join(directory, output), arcname=output)
            else:
                print(f"{output} does not exist", file=sys.stderr)
    with open("/content/.colabofy/status", "w") as f:
        f.write(str(status))
EOS
} > "${work}/bundle.py"

colab new -s "$session" --gpu "$gpu"

cleanup() {
  if [ -S "${work}/ssh.socket" ]; then
    ssh -S "${work}/ssh.socket" -O exit "$session" 2> /dev/null || true
  fi
  rm -rf "$work"
  colab stop -s "$session" || echo "colab stop -s ${session} failed" 1>&2
}

trap cleanup EXIT

if [ -n "$files" ]; then
  colab drivemount -s "$session"
fi

if [ -n "$port" ]; then
  colab=$(command -v colab)
  ssh -f -N -M -S "${work}/ssh.socket" \
    -o ExitOnForwardFailure=yes \
    -o ProxyCommand="${colab} ssh --proxy-mode -s ${session}" \
    -o User=root \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -L "127.0.0.1:${port}:localhost:${port}" \
    "$session"
fi

colab exec -s "$session" -f "${work}/bundle.py" --timeout "$timeout"
colab download -s "$session" /content/.colabofy/status "${work}/status"
colab download -s "$session" /content/.colabofy/output.tar.gz "${work}/output.tar.gz"
tar -xzf "${work}/output.tar.gz"

status=$(cat "${work}/status")
exit "$status"
