#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly GITHUB_USER=kaidotio
readonly VM_CPUS=8
readonly VM_MEMORY=32GB
readonly VM_DISK_SIZE=500G
readonly BOOT_SECONDS=900
readonly POLL_SECONDS=10
readonly IMAGE_URL=https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img

usage() {
  cat <<EOS
Usage:
   minikube-add-hyperv-worker.sh [<node>|<windows-user>@<windows-host>]

Create each configured Hyper-V worker as an Ubuntu 24.04 VM over Windows SSH, then wait for minikube-worker.sh to join it
Select one node or Windows SSH target to create only that worker
The Windows account needs Hyper-V Administrators or Administrators, OpenSSH Server with this host's key, and the configured external virtual switch
Skip each node that already exists; delete its Kubernetes node before recreating its VM
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

readonly CACHE_DIRECTORY="${XDG_CACHE_HOME:-${HOME}/.cache}/minikube-add-hyperv-worker"

for worker in "${hyperv_workers[@]}"; do
  (
    read -r worker_target target vm_switch _ <<< "$worker"
    worker_host="${worker_target#*@}"
    vm_name="${worker_host%.local}"

    node=$(minikube kubectl -- get node "$vm_name" --ignore-not-found -o name)
    if [ -n "$node" ]; then
      echo "Node ${vm_name} already exists"
      exit 0
    fi
    if getent ahostsv4 "${vm_name}.local" > /dev/null; then
      echo "${vm_name}.local already answers on the LAN" 1>&2
      exit 1
    fi

    public_keys=$(curl -fsSL --retry 5 --retry-all-errors "https://github.com/${GITHUB_USER}.keys")

    mkdir -p "$CACHE_DIRECTORY"
    if [ ! -f "${CACHE_DIRECTORY}/${IMAGE_URL##*/}" ]; then
      curl -fsSL --retry 5 --retry-all-errors -o "${CACHE_DIRECTORY}/${IMAGE_URL##*/}.part" "$IMAGE_URL"
      mv "${CACHE_DIRECTORY}/${IMAGE_URL##*/}.part" "${CACHE_DIRECTORY}/${IMAGE_URL##*/}"
    fi

    work=$(mktemp -d -p "$CACHE_DIRECTORY")
    host_key_directory=$(mktemp -d "${XDG_RUNTIME_DIR:-/tmp}/minikube-add-hyperv-worker.XXXXXX")
    trap 'rm -rf "$work" "$host_key_directory"' EXIT
    ssh-keygen -q -t ed25519 -N "" -C "" -f "${host_key_directory}/ssh_host_ed25519_key"

    echo "=== disk: ${VM_DISK_SIZE} VHDX from ${IMAGE_URL##*/}"
    /usr/bin/qemu-img convert -f qcow2 -O raw "${CACHE_DIRECTORY}/${IMAGE_URL##*/}" "${work}/disk.raw"
    truncate -s "$VM_DISK_SIZE" "${work}/disk.raw"
    # https://learn.microsoft.com/en-us/windows-server/virtualization/hyper-v/best-practices-for-running-linux-on-hyper-v#tuning-linux-file-systems-on-dynamic-vhdx-files
    /usr/bin/qemu-img convert -f raw -O vhdx -o subformat=dynamic,block_size=1M "${work}/disk.raw" "${work}/${vm_name}.vhdx"
    rm "${work}/disk.raw"

    echo "=== seed: cloud-init NoCloud for ${vm_name}"
    mkdir "${work}/seed"
    cat <<EOS > "${work}/seed/meta-data"
instance-id: ${vm_name}
local-hostname: ${vm_name}
EOS
    cat <<EOS > "${work}/seed/user-data"
#cloud-config
hostname: ${vm_name}
ssh_deletekeys: true
ssh_genkeytypes: []
ssh_keys:
  ed25519_private: |
$(sed 's/^/    /' "${host_key_directory}/ssh_host_ed25519_key")
  ed25519_public: $(< "${host_key_directory}/ssh_host_ed25519_key.pub")
ssh_authorized_keys:
$(sed 's/^/  - /' <<< "$public_keys")
package_update: true
packages:
  - avahi-daemon
  - libnss-mdns
EOS
    xorriso -as mkisofs -output "${work}/${vm_name}-seed.iso" -volid CIDATA -joliet -rock "${work}/seed/user-data" "${work}/seed/meta-data" 2> /dev/null

    # Windows PowerShell 5.1 reads a script without a BOM in the ANSI code page
    printf '\xef\xbb\xbf' > "${work}/${vm_name}.ps1"
    cat <<EOS >> "${work}/${vm_name}.ps1"
\$ErrorActionPreference = 'Stop'
\$directory = (Get-VMHost).VirtualHardDiskPath
New-Item -ItemType Directory -Force \$directory | Out-Null
Get-VMSwitch -Name '${vm_switch}' | Out-Null
if (Get-VM -Name '${vm_name}' -ErrorAction SilentlyContinue) { throw "VM ${vm_name} already exists" }
if (Test-Path "\$directory\\${vm_name}.vhdx") { throw "\$directory\\${vm_name}.vhdx already exists" }
# Move-Item would keep the ACL of the user's home, while a copy inherits the directory's
Copy-Item -Force "\$HOME\\${vm_name}.vhdx" "\$directory\\${vm_name}.vhdx"
Copy-Item -Force "\$HOME\\${vm_name}-seed.iso" "\$directory\\${vm_name}-seed.iso"
Remove-Item "\$HOME\\${vm_name}.vhdx", "\$HOME\\${vm_name}-seed.iso"
New-VM -Name '${vm_name}' -Generation 2 -MemoryStartupBytes ${VM_MEMORY} -VHDPath "\$directory\\${vm_name}.vhdx" -SwitchName '${vm_switch}' | Out-Null
Set-VMProcessor -VMName '${vm_name}' -Count ${VM_CPUS}
Set-VMMemory -VMName '${vm_name}' -DynamicMemoryEnabled \$false
# https://learn.microsoft.com/en-us/troubleshoot/windows-server/virtualization/linux-virtual-machine-issue-hyper-v
Set-VMFirmware -VMName '${vm_name}' -SecureBootTemplate MicrosoftUEFICertificateAuthority
Add-VMDvdDrive -VMName '${vm_name}' -Path "\$directory\\${vm_name}-seed.iso"
Set-VM -Name '${vm_name}' -AutomaticStartAction Start -AutomaticStopAction ShutDown -AutomaticCheckpointsEnabled \$false
Start-VM -Name '${vm_name}'
EOS

    # The seed carries the VM's private host key
    printf '\xef\xbb\xbf' > "${work}/${vm_name}-cleanup.ps1"
    cat <<EOS >> "${work}/${vm_name}-cleanup.ps1"
\$ErrorActionPreference = 'Stop'
Get-VMDvdDrive -VMName '${vm_name}' | Remove-VMDvdDrive
Remove-Item (Join-Path (Get-VMHost).VirtualHardDiskPath '${vm_name}-seed.iso')
EOS

    echo "=== Hyper-V: VM ${vm_name} on switch ${vm_switch}"
    scp "${work}/${vm_name}.vhdx" "${work}/${vm_name}-seed.iso" "${work}/${vm_name}.ps1" "${work}/${vm_name}-cleanup.ps1" "${target}:"
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File ${vm_name}.ps1"

    echo "=== waiting for ${vm_name}.local"
    ssh-keygen -R "${vm_name}.local" 2> /dev/null || true
    echo "${vm_name}.local $(< "${host_key_directory}/ssh_host_ed25519_key.pub")" >> ~/.ssh/known_hosts
    deadline=$(($(date +%s) + BOOT_SECONDS))
    until [ "$(date +%s)" -ge "$deadline" ] || ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=5 "$worker_target" true 2> /dev/null; do
      sleep "$POLL_SECONDS"
    done
    if ! ssh -o BatchMode=yes -o ConnectTimeout=5 "$worker_target" true; then
      echo "${worker_target} did not accept SSH within ${BOOT_SECONDS}s" 1>&2
      exit 1
    fi
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File ${vm_name}-cleanup.ps1"
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Remove-Item ${vm_name}.ps1, ${vm_name}-cleanup.ps1\""

    echo "=== waiting for minikube-worker.timer to join ${vm_name}"
    deadline=$(($(date +%s) + BOOT_SECONDS))
    until [ "$(date +%s)" -ge "$deadline" ] || minikube kubectl -- get node "$vm_name" > /dev/null 2>&1; do
      sleep "$POLL_SECONDS"
    done
    if ! minikube kubectl -- get node "$vm_name" > /dev/null; then
      echo "minikube-worker.timer did not join ${vm_name} within ${BOOT_SECONDS}s, see journalctl -u minikube-worker.service" 1>&2
      exit 1
    fi
  )
done
