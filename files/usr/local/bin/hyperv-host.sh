#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly DRAIN_TIMEOUT=5m
readonly READY_TIMEOUT=15m
readonly BOOT_SECONDS=900
readonly NOT_READY_SECONDS=300
readonly POLL_SECONDS=5
readonly RELOCATION_TARGET=minikube

usage() {
  cat <<EOS
Usage:
   hyperv-host.sh start|stop|on|off|shutdown [<node>|<windows-user>@<windows-host>]

Operate on every configured Hyper-V worker, or only the selected node or Windows SSH target

off: relocate the pods annotated with relocatable-volume-controller.kaidotio.github.io/volumes to ${RELOCATION_TARGET} and drain the worker if it is Ready, stop hippocampus-compose.service, if setup/hyperv-host/wsl.sh --nvidia installed it, in its host WSL distribution and shut down the VM, keeping both from starting at boot
stop: relocate those pods, drain the worker and stop the VM and hippocampus-compose.service without changing their automatic startup settings or shutting down Windows
start: wait for Windows SSH and start the VM and hippocampus-compose.service only where their automatic startup settings are enabled, without changing those settings
on: start them again at once and at boot, uncordon the worker and wait until it is Ready, after which minikube-worker.sh relocates those pods back
shutdown: relocate those pods and drain the worker if it is Ready, shut down its Windows host and uncordon the worker once it is NotReady, so that it takes pods again when it starts at the next boot unless off kept it stopped
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

if [ "${#args[@]}" -lt 1 ] || [ "${#args[@]}" -gt 2 ]; then
  usage 1>&2
  exit 1
fi

case "${args[0]}" in
  start|stop|on|off|shutdown)
    ;;
  *)
    usage 1>&2
    exit 1
    ;;
esac

source /usr/local/lib/hyperv-host/lib.sh
select_hyperv_workers "${args[1]:-}"

pids=()
terminate_workers() {
  local running_workers pid
  running_workers=$(jobs -pr)
  for pid in ${running_workers}; do
    kill "$pid" 2> /dev/null || true
  done
  for pid in "${pids[@]}"; do
    wait "$pid" || true
  done
  exit 1
}
trap terminate_workers TERM INT

for worker in "${hyperv_workers[@]}"; do
  (
    read -r worker_target windows_target _ <<< "$worker"
    worker_host="${worker_target#*@}"
    vm_name="${worker_host%.local}"
    wsl_target="root@${windows_target#*@}"

    case "${args[0]}" in
      off|stop|shutdown)
        status=$(minikube kubectl -- get node "$vm_name" --ignore-not-found -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
        if [ "${args[0]}" = stop ] && [ "$status" != True ]; then
          exit 0
        fi
        # The relocation never finishes on a node that is not Ready: active in the Reconcile of cluster/applications/relocatable-volume-controller/internal/controllers/relocation_controller.go
        if [ "$status" = True ]; then
          minikube kubectl -- cordon "$vm_name"
          if ! relocate_volumes "$vm_name" "$RELOCATION_TARGET" || ! minikube kubectl -- drain "$vm_name" --ignore-daemonsets --delete-emptydir-data --timeout="$DRAIN_TIMEOUT"; then
            minikube kubectl -- uncordon "$vm_name"
            exit 1
          fi
        fi
        if [ "${args[0]}" = shutdown ]; then
          trap 'minikube kubectl -- uncordon "$vm_name"' EXIT
          # Without -Force, the shutdown fails with 1191 while another user is signed in: https://learn.microsoft.com/en-us/windows/win32/debug/system-error-codes--1000-1299-
          # A forced shutdown leaves its return value indeterminate: https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32shutdown-method-in-class-win32-operatingsystem
          ssh "$windows_target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Stop-Computer -Force\"" || true
          # The scheduler keeps pods off a NotReady node: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/#taint-nodes-by-condition
          # The node controller sets Ready to Unknown rather than False once the kubelet stops posting: https://kubernetes.io/docs/reference/node/node-status/#condition
          deadline=$(($(date +%s) + NOT_READY_SECONDS))
          status=$(minikube kubectl -- get node "$vm_name" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
          until [ "$(date +%s)" -ge "$deadline" ] || [ "$status" != True ]; do
            sleep "$POLL_SECONDS"
            status=$(minikube kubectl -- get node "$vm_name" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
          done
          if [ "$status" = True ]; then
            echo "Node ${vm_name} stayed Ready for ${NOT_READY_SECONDS}s after the shutdown of ${windows_target#*@}" 1>&2
            exit 1
          fi
        else
          if [ "${args[0]}" = off ]; then
            ssh -o StrictHostKeyChecking=accept-new -p "$HYPERV_WSL_PORT" "$wsl_target" "if [ -f /etc/systemd/system/hippocampus-compose.service ]; then systemctl disable --now hippocampus-compose.service; fi"
            ssh "$windows_target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Set-VM -Name ${vm_name} -AutomaticStartAction Nothing\""
          else
            ssh -o StrictHostKeyChecking=accept-new -p "$HYPERV_WSL_PORT" "$wsl_target" "if [ -f /etc/systemd/system/hippocampus-compose.service ]; then systemctl stop hippocampus-compose.service; fi"
          fi
          ssh "$windows_target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Stop-VM -Name ${vm_name}\""
        fi
        ;;
      on|start)
        if [ "${args[0]}" = start ]; then
          node=$(minikube kubectl -- get node "$vm_name" --ignore-not-found -o name)
          if [ -z "$node" ]; then
            exit 0
          fi
          deadline=$(($(date +%s) + BOOT_SECONDS))
          until [ "$(date +%s)" -ge "$deadline" ] || ssh -o BatchMode=yes -o ConnectTimeout=5 "$windows_target" "exit 0"; do
            sleep "$POLL_SECONDS"
          done
          automatic_start=$(ssh -o BatchMode=yes -o ConnectTimeout=5 "$windows_target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"(Get-VM -Name ${vm_name} -ErrorAction Stop).AutomaticStartAction\"")
          if [ "$(tr -d '\r\n' <<< "$automatic_start")" = Nothing ]; then
            exit 0
          fi
          until [ "$(date +%s)" -ge "$deadline" ] || ssh -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new -p "$HYPERV_WSL_PORT" "$wsl_target" true; do
            sleep "$POLL_SECONDS"
          done
          ssh -o StrictHostKeyChecking=accept-new -p "$HYPERV_WSL_PORT" "$wsl_target" "if [ -f /etc/systemd/system/hippocampus-compose.service ] && systemctl is-enabled --quiet hippocampus-compose.service; then systemctl start hippocampus-compose.service; fi"
        else
          # AutomaticStartAction in files/usr/local/bin/minikube-add-hyperv-worker.sh
          ssh "$windows_target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Set-VM -Name ${vm_name} -AutomaticStartAction Start\""
          ssh -o StrictHostKeyChecking=accept-new -p "$HYPERV_WSL_PORT" "$wsl_target" "if [ -f /etc/systemd/system/hippocampus-compose.service ]; then systemctl enable --now hippocampus-compose.service; fi"
        fi
        minikube kubectl -- uncordon "$vm_name"
        ssh "$windows_target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"if ((Get-VM -Name ${vm_name} -ErrorAction Stop).State -eq 'Off') { Start-VM -Name ${vm_name} -ErrorAction Stop }\""
        if [ "${args[0]}" = on ]; then
          minikube kubectl -- wait --for=condition=Ready "node/${vm_name}" --timeout="$READY_TIMEOUT"
        fi
        ;;
    esac
  ) &
  pids+=($!)
done

status=0
for pid in "${pids[@]}"; do
  wait "$pid" || status=1
done
exit "$status"
