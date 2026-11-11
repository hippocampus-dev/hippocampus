#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

usage() {
  cat <<EOS
Usage:
   setup-hyperv-host-wsl.sh [<node>|<windows-user>@<windows-host>]

Send the compose definition of stable-diffusion-webui-forge to each configured Windows host whose WSL distribution carries hippocampus-compose.service from setup/hyperv-host/wsl.sh --nvidia, then restart it
Select one node or Windows SSH target to deploy only to that host
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

if [ "${#args[@]}" -gt 1 ]; then
  usage 1>&2
  exit 1
fi

source /usr/local/lib/hyperv-host/lib.sh
select_hyperv_workers "${args[0]:-}"

cd /opt/hippocampus
for worker in "${hyperv_workers[@]}"; do
  read -r _ windows_target _ <<< "$worker"
  target="root@${windows_target#*@}"
  installed=$(ssh -o StrictHostKeyChecking=accept-new -p "$HYPERV_WSL_PORT" "$target" "if [ -f /etc/systemd/system/hippocampus-compose.service ]; then echo installed; fi")
  if [ -z "$installed" ]; then
    continue
  fi
  if [ ! -f docker-compose/stable-diffusion-webui-forge/styles.csv ]; then
    echo "docker-compose/stable-diffusion-webui-forge/styles.csv is missing, so decrypt it with bin/decrypt.sh" 1>&2
    exit 1
  fi
  tar -c docker-compose.yaml docker-compose/docker-compose.*.yaml docker-compose/stable-diffusion-webui-forge | ssh -o StrictHostKeyChecking=accept-new -p "$HYPERV_WSL_PORT" "$target" "rm -rf /opt/hippocampus && mkdir /opt/hippocampus && tar -x -f - -C /opt/hippocampus && systemctl restart hippocampus-compose.service"
done
