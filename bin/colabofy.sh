#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   colabofy.sh [options] -- COMMAND...

Options:
   --gpu GPU          Accelerator to allocate: T4, L4, G4, H100 or A100 (default: T4)
   --port PORT        Forward 127.0.0.1:PORT to PORT on the runtime while COMMAND runs
   --output PATH      Copy PATH back into the current directory after COMMAND exits (repeatable)
   --timeout SECONDS  Longest silence from COMMAND that colab exec tolerates (default: 86400)
   -h, --help         Show this help
EOS
}

gpu=T4
port=""
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
  usage >&2
  exit 1
fi

session="colabofy-${RANDOM}"
work=$(mktemp -d "${XDG_RUNTIME_DIR:-/tmp}/colabofy.XXXXXX")
trap 'rm -rf "$work"' EXIT

git ls-files -z --cached --others --exclude-standard | while IFS= read -r -d '' file; do
  # .env points at endpoints on this machine (HTTP_PROXY=http://127.0.0.1:18080 in cluster/applications/meegeren/.env)
  [ -f "$file" ] && [ "${file##*/}" != ".env" ] || continue
  printf '%s\0' "$file"
done | tar --null -T - -czf "${work}/archive.tar.gz"

archive=$(base64 -w 0 "${work}/archive.tar.gz")
encoded_command=$(jq -cn '$ARGS.positional' --args -- "$@" | base64 -w 0)
encoded_outputs=$(jq -cn '$ARGS.positional' --args -- "${outputs[@]}" | base64 -w 0)
encoded_project=$(printf '%s' "${PWD##*/}" | base64 -w 0)
{
  printf 'ARCHIVE = "%s"\n' "$archive"
  printf 'COMMAND = "%s"\n' "$encoded_command"
  printf 'OUTPUTS = "%s"\n' "$encoded_outputs"
  printf 'PROJECT = "%s"\n' "$encoded_project"
  cat <<'EOS'

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
    with tarfile.open("/content/colabofy-output.tar.gz", "w:gz") as archive:
        for output in json.loads(base64.b64decode(OUTPUTS)):
            if os.path.exists(os.path.join(directory, output)):
                archive.add(os.path.join(directory, output), arcname=output)
            else:
                print(f"{output} does not exist", file=sys.stderr)
    with open("/content/colabofy-status", "w") as f:
        f.write(str(status))
EOS
} > "${work}/bundle.py"

colab new -s "$session" --gpu "$gpu"
trap 'rm -rf "$work"; colab stop -s "$session" || echo "colab stop -s ${session} failed" >&2' EXIT

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
  trap 'ssh -S "${work}/ssh.socket" -O exit "$session" 2> /dev/null || true; rm -rf "$work"; colab stop -s "$session" || echo "colab stop -s ${session} failed" >&2' EXIT
fi

colab exec -s "$session" -f "${work}/bundle.py" --timeout "$timeout"
colab download -s "$session" /content/colabofy-status "${work}/status"
colab download -s "$session" /content/colabofy-output.tar.gz "${work}/output.tar.gz"
tar -xzf "${work}/output.tar.gz"

status=$(cat "${work}/status")
exit "$status"
