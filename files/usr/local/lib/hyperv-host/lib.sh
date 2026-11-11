#!/usr/bin/env bash
# Sourced library (no set -e / trap of its own; the sourcing script owns those).

# Each entry is "<worker SSH target> <Windows SSH target> <virtual switch> [<teleput edge>]".
HYPERV_WORKERS=(
  "ubuntu@hyperv-worker.local kai@hyperv-host.local External left"
)
HYPERV_WSL_PORT=2222
# The annotations cluster/applications/relocatable-volume-controller reads
RELOCATABLE_ANNOTATION=relocatable-volume-controller.kaidotio.github.io/volumes
RELOCATE_TO_ANNOTATION=relocatable-volume-controller.kaidotio.github.io/relocate-to
RELOCATE_SECONDS=1200
RELOCATE_POLL_SECONDS=5

relocate_volumes() {
  local source="$1" target="$2"
  local deadline node relocating
  minikube kubectl -- annotate node "$source" "${RELOCATE_TO_ANNOTATION}=${target}" --overwrite || return 1
  deadline=$(($(date +%s) + RELOCATE_SECONDS))
  until [ "$(date +%s)" -ge "$deadline" ]; do
    if node=$(minikube kubectl -- get node "$source" -o json); then
      relocating=$(jq -r --arg key "$RELOCATE_TO_ANNOTATION" '.metadata.annotations[$key] // ""' <<< "$node")
      if [ -z "$relocating" ]; then
        return 0
      fi
    fi
    sleep "$RELOCATE_POLL_SECONDS"
  done
  minikube kubectl -- annotate node "$source" "${RELOCATE_TO_ANNOTATION}-"
  echo "Volumes on ${source} did not reach ${target} within ${RELOCATE_SECONDS}s" 1>&2
  return 1
}

select_hyperv_workers() {
  local selected="${1:-}"
  local worker target windows_target host
  hyperv_workers=()
  for worker in "${HYPERV_WORKERS[@]}"; do
    read -r target windows_target _ <<< "$worker"
    host="${target#*@}"
    if [ -z "$selected" ] || [ "$selected" = "${host%.local}" ] || [ "$selected" = "$windows_target" ]; then
      hyperv_workers+=("$worker")
    fi
  done
  if [ -n "$selected" ] && [ "${#hyperv_workers[@]}" -eq 0 ]; then
    echo "Unknown Hyper-V worker or Windows SSH target: ${selected}" 1>&2
    return 1
  fi
}
