#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

GITHUB_USER=kaidotio
VM_NAME=hyperv-worker
VM_SWITCH=External
VM_CPUS=8
VM_MEMORY=32GB
VM_DISK_SIZE=500G
BOOT_SECONDS=900
POLL_SECONDS=10
IMAGE_URL=https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img

function usage() {
  cat <<EOS
Usage:
   minikube-add-hyperv-worker.sh <windows-user>@<windows-host>

Create the Ubuntu 24.04 VM ${VM_NAME} on the Hyper-V of the Windows host over SSH, then wait for minikube-worker.sh to join it
The Windows account needs Hyper-V Administrators or Administrators, OpenSSH Server with this host's key, and the external virtual switch ${VM_SWITCH}
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

if [ "${#args[@]}" -ne 1 ]; then
  usage >&2
  exit 1
fi
target="${args[0]}"
if getent ahostsv4 "${VM_NAME}.local" > /dev/null; then
  echo "${VM_NAME}.local already answers on the LAN" >&2
  exit 1
fi
node=$(minikube kubectl -- get node "$VM_NAME" --ignore-not-found -o name)
if [ -n "$node" ]; then
  echo "Node ${VM_NAME} already exists, so minikube-worker.sh would skip joining the new VM; delete it with minikube kubectl -- delete node ${VM_NAME}" >&2
  exit 1
fi

public_keys=$(curl -fsSL --retry 5 --retry-all-errors "https://github.com/${GITHUB_USER}.keys")

cache="${XDG_CACHE_HOME:-$HOME/.cache}/minikube-add-hyperv-worker"
mkdir -p "$cache"
if [ ! -f "${cache}/${IMAGE_URL##*/}" ]; then
  curl -fsSL --retry 5 --retry-all-errors -o "${cache}/${IMAGE_URL##*/}.part" "$IMAGE_URL"
  mv "${cache}/${IMAGE_URL##*/}.part" "${cache}/${IMAGE_URL##*/}"
fi

work=$(mktemp -d -p "$cache")
host_key_directory=$(mktemp -d "${XDG_RUNTIME_DIR:-/tmp}/minikube-add-hyperv-worker.XXXXXX")
trap 'rm -rf "$work" "$host_key_directory"' EXIT
ssh-keygen -q -t ed25519 -N '' -C '' -f "${host_key_directory}/ssh_host_ed25519_key"

echo "=== disk: ${VM_DISK_SIZE} VHDX from ${IMAGE_URL##*/}"
/usr/bin/qemu-img convert -f qcow2 -O raw "${cache}/${IMAGE_URL##*/}" "${work}/disk.raw"
truncate -s "$VM_DISK_SIZE" "${work}/disk.raw"
# https://learn.microsoft.com/en-us/windows-server/virtualization/hyper-v/best-practices-for-running-linux-on-hyper-v#tuning-linux-file-systems-on-dynamic-vhdx-files
/usr/bin/qemu-img convert -f raw -O vhdx -o subformat=dynamic,block_size=1M "${work}/disk.raw" "${work}/${VM_NAME}.vhdx"
rm "${work}/disk.raw"

echo "=== seed: cloud-init NoCloud for ${VM_NAME}"
mkdir "${work}/seed"
cat <<EOS > "${work}/seed/meta-data"
instance-id: ${VM_NAME}
local-hostname: ${VM_NAME}
EOS
cat <<EOS > "${work}/seed/user-data"
#cloud-config
hostname: ${VM_NAME}
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
xorriso -as mkisofs -output "${work}/${VM_NAME}-seed.iso" -volid CIDATA -joliet -rock "${work}/seed/user-data" "${work}/seed/meta-data" 2> /dev/null

# Windows PowerShell 5.1 reads a script without a BOM in the ANSI code page
printf '\xef\xbb\xbf' > "${work}/${VM_NAME}.ps1"
cat <<EOS >> "${work}/${VM_NAME}.ps1"
\$ErrorActionPreference = 'Stop'
\$directory = (Get-VMHost).VirtualHardDiskPath
New-Item -ItemType Directory -Force \$directory | Out-Null
Get-VMSwitch -Name '${VM_SWITCH}' | Out-Null
if (Get-VM -Name '${VM_NAME}' -ErrorAction SilentlyContinue) { throw "VM ${VM_NAME} already exists" }
if (Test-Path "\$directory\\${VM_NAME}.vhdx") { throw "\$directory\\${VM_NAME}.vhdx already exists" }
# Move-Item would keep the ACL of the user's home, while a copy inherits the directory's
Copy-Item -Force "\$HOME\\${VM_NAME}.vhdx" "\$directory\\${VM_NAME}.vhdx"
Copy-Item -Force "\$HOME\\${VM_NAME}-seed.iso" "\$directory\\${VM_NAME}-seed.iso"
Remove-Item "\$HOME\\${VM_NAME}.vhdx", "\$HOME\\${VM_NAME}-seed.iso"
New-VM -Name '${VM_NAME}' -Generation 2 -MemoryStartupBytes ${VM_MEMORY} -VHDPath "\$directory\\${VM_NAME}.vhdx" -SwitchName '${VM_SWITCH}' | Out-Null
Set-VMProcessor -VMName '${VM_NAME}' -Count ${VM_CPUS}
Set-VMMemory -VMName '${VM_NAME}' -DynamicMemoryEnabled \$false
# https://learn.microsoft.com/en-us/troubleshoot/windows-server/virtualization/linux-virtual-machine-issue-hyper-v
Set-VMFirmware -VMName '${VM_NAME}' -SecureBootTemplate MicrosoftUEFICertificateAuthority
Add-VMDvdDrive -VMName '${VM_NAME}' -Path "\$directory\\${VM_NAME}-seed.iso"
Set-VM -Name '${VM_NAME}' -AutomaticStartAction Start -AutomaticStopAction ShutDown -AutomaticCheckpointsEnabled \$false
Start-VM -Name '${VM_NAME}'
(Get-NetAdapter -InterfaceDescription (Get-VMSwitch -Name '${VM_SWITCH}').NetAdapterInterfaceDescription).MacAddress
EOS

# The seed carries the VM's private host key
printf '\xef\xbb\xbf' > "${work}/${VM_NAME}-cleanup.ps1"
cat <<EOS >> "${work}/${VM_NAME}-cleanup.ps1"
\$ErrorActionPreference = 'Stop'
Get-VMDvdDrive -VMName '${VM_NAME}' | Remove-VMDvdDrive
Remove-Item (Join-Path (Get-VMHost).VirtualHardDiskPath '${VM_NAME}-seed.iso')
EOS

echo "=== Hyper-V: VM ${VM_NAME} on switch ${VM_SWITCH}"
scp "${work}/${VM_NAME}.vhdx" "${work}/${VM_NAME}-seed.iso" "${work}/${VM_NAME}.ps1" "${work}/${VM_NAME}-cleanup.ps1" "${target}:"
output=$(ssh "$target" powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "${VM_NAME}.ps1")
mac=$(tr -d '\r' <<< "$output" | awk 'NF {line = $0} END {gsub("-", ":", line); print tolower(line)}')

echo "=== waiting for ${VM_NAME}.local"
ssh-keygen -R "${VM_NAME}.local" 2> /dev/null || true
echo "${VM_NAME}.local $(< "${host_key_directory}/ssh_host_ed25519_key.pub")" >> ~/.ssh/known_hosts
deadline=$(($(date +%s) + BOOT_SECONDS))
until [ "$(date +%s)" -ge "$deadline" ] || ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=5 "ubuntu@${VM_NAME}.local" true 2> /dev/null; do
  sleep "$POLL_SECONDS"
done
if ! ssh -o BatchMode=yes -o ConnectTimeout=5 "ubuntu@${VM_NAME}.local" true; then
  echo "ubuntu@${VM_NAME}.local did not accept SSH within ${BOOT_SECONDS}s" >&2
  exit 1
fi
ssh "$target" powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "${VM_NAME}-cleanup.ps1"
ssh "$target" powershell -NoProfile -NonInteractive -Command "Remove-Item ${VM_NAME}.ps1, ${VM_NAME}-cleanup.ps1"

echo "=== waiting for minikube-worker.timer to join ${VM_NAME}"
deadline=$(($(date +%s) + BOOT_SECONDS))
until [ "$(date +%s)" -ge "$deadline" ] || minikube kubectl -- get node "$VM_NAME" > /dev/null 2>&1; do
  sleep "$POLL_SECONDS"
done
if ! minikube kubectl -- get node "$VM_NAME" > /dev/null; then
  echo "minikube-worker.timer did not join ${VM_NAME} within ${BOOT_SECONDS}s, see journalctl -u minikube-worker.service and annotate it with wake-on-lan.kaidotio.github.io/mac=${mac} once joined" >&2
  exit 1
fi
minikube kubectl -- annotate node "$VM_NAME" "wake-on-lan.kaidotio.github.io/mac=${mac}" --overwrite
