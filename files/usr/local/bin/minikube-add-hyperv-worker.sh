#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

VM_CPUS=8
VM_MEMORY=16GB
VM_DISK_SIZE=100G
BOOT_SECONDS=900
POLL_SECONDS=10
IMAGE_URL=https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2

function usage() {
  cat <<EOS
Usage:
   minikube-add-hyperv-worker.sh <windows-user>@<windows-host> <vm-name> <switch-name>

Create a Debian 12 VM on the Hyper-V of the Windows host over SSH, then join it with minikube-worker.sh
The Windows account needs Hyper-V Administrators or Administrators, OpenSSH Server with this host's key, and an external virtual switch
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

if [ "${#args[@]}" -ne 3 ]; then
  usage >&2
  exit 1
fi
target="${args[0]}"
name="${args[1]}"
switch="${args[2]}"
if [[ ! "$name" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$ ]]; then
  echo "The VM name becomes its hostname and must match ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$" >&2
  exit 1
fi
if [[ "$switch" =~ [\'‘’‚‛] ]]; then
  echo "The switch name cannot contain a single quote, which PowerShell also reads from ‘’‚‛" >&2
  exit 1
fi
if getent ahostsv4 "${name}.local" > /dev/null; then
  echo "${name}.local already answers on the LAN" >&2
  exit 1
fi

identity_files=$(ssh -G "debian@${name}.local" | awk '$1 == "identityfile" {print $2}')
public_key=""
for identity_file in $identity_files; do
  identity_file="${identity_file/#\~/$HOME}"
  if [ -f "${identity_file}.pub" ]; then
    public_key=$(< "${identity_file}.pub")
    break
  fi
done
if [ -z "$public_key" ]; then
  echo "No public key beside the identity files ssh uses for debian@${name}.local" >&2
  exit 1
fi

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
/usr/bin/qemu-img convert -f raw -O vhdx -o subformat=dynamic,block_size=1M "${work}/disk.raw" "${work}/${name}.vhdx"
rm "${work}/disk.raw"

echo "=== seed: cloud-init NoCloud for ${name}"
mkdir "${work}/seed"
cat <<EOS > "${work}/seed/meta-data"
instance-id: ${name}
local-hostname: ${name}
EOS
cat <<EOS > "${work}/seed/user-data"
#cloud-config
hostname: ${name}
ssh_deletekeys: true
ssh_genkeytypes: []
ssh_keys:
  ed25519_private: |
$(sed 's/^/    /' "${host_key_directory}/ssh_host_ed25519_key")
  ed25519_public: $(< "${host_key_directory}/ssh_host_ed25519_key.pub")
ssh_authorized_keys:
  - ${public_key}
package_update: true
packages:
  - avahi-daemon
  - libnss-mdns
EOS
xorriso -as mkisofs -output "${work}/${name}-seed.iso" -volid CIDATA -joliet -rock "${work}/seed/user-data" "${work}/seed/meta-data" 2> /dev/null

# Windows PowerShell 5.1 reads a script without a BOM in the ANSI code page
printf '\xef\xbb\xbf' > "${work}/${name}.ps1"
cat <<EOS >> "${work}/${name}.ps1"
\$ErrorActionPreference = 'Stop'
\$directory = (Get-VMHost).VirtualHardDiskPath
New-Item -ItemType Directory -Force \$directory | Out-Null
Get-VMSwitch -Name '${switch}' | Out-Null
if (Get-VM -Name '${name}' -ErrorAction SilentlyContinue) { throw "VM ${name} already exists" }
if (Test-Path "\$directory\\${name}.vhdx") { throw "\$directory\\${name}.vhdx already exists" }
# Move-Item would keep the ACL of the user's home, while a copy inherits the directory's
Copy-Item -Force "\$HOME\\${name}.vhdx" "\$directory\\${name}.vhdx"
Copy-Item -Force "\$HOME\\${name}-seed.iso" "\$directory\\${name}-seed.iso"
Remove-Item "\$HOME\\${name}.vhdx", "\$HOME\\${name}-seed.iso"
New-VM -Name '${name}' -Generation 2 -MemoryStartupBytes ${VM_MEMORY} -VHDPath "\$directory\\${name}.vhdx" -SwitchName '${switch}' | Out-Null
Set-VMProcessor -VMName '${name}' -Count ${VM_CPUS}
Set-VMMemory -VMName '${name}' -DynamicMemoryEnabled \$false
# https://learn.microsoft.com/en-us/troubleshoot/windows-server/virtualization/linux-virtual-machine-issue-hyper-v
Set-VMFirmware -VMName '${name}' -SecureBootTemplate MicrosoftUEFICertificateAuthority
Add-VMDvdDrive -VMName '${name}' -Path "\$directory\\${name}-seed.iso"
Set-VM -Name '${name}' -AutomaticStartAction Start -AutomaticStopAction ShutDown -AutomaticCheckpointsEnabled \$false
Start-VM -Name '${name}'
(Get-NetAdapter -InterfaceDescription (Get-VMSwitch -Name '${switch}').NetAdapterInterfaceDescription).MacAddress
EOS

# The seed carries the VM's private host key
printf '\xef\xbb\xbf' > "${work}/${name}-cleanup.ps1"
cat <<EOS >> "${work}/${name}-cleanup.ps1"
\$ErrorActionPreference = 'Stop'
Get-VMDvdDrive -VMName '${name}' | Remove-VMDvdDrive
Remove-Item (Join-Path (Get-VMHost).VirtualHardDiskPath '${name}-seed.iso')
EOS

echo "=== Hyper-V: VM ${name} on switch ${switch}"
scp "${work}/${name}.vhdx" "${work}/${name}-seed.iso" "${work}/${name}.ps1" "${work}/${name}-cleanup.ps1" "${target}:"
output=$(ssh "$target" powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "${name}.ps1")
mac=$(tr -d '\r' <<< "$output" | awk 'NF {line = $0} END {gsub("-", ":", line); print tolower(line)}')

echo "=== waiting for ${name}.local"
ssh-keygen -R "${name}.local" 2> /dev/null || true
echo "${name}.local $(< "${host_key_directory}/ssh_host_ed25519_key.pub")" >> ~/.ssh/known_hosts
deadline=$(($(date +%s) + BOOT_SECONDS))
until [ "$(date +%s)" -ge "$deadline" ] || ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=5 "debian@${name}.local" true 2> /dev/null; do
  sleep "$POLL_SECONDS"
done
if ! ssh -o BatchMode=yes -o ConnectTimeout=5 "debian@${name}.local" true; then
  echo "debian@${name}.local did not accept SSH within ${BOOT_SECONDS}s" >&2
  exit 1
fi
ssh "$target" powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "${name}-cleanup.ps1"
ssh "$target" powershell -NoProfile -NonInteractive -Command "Remove-Item ${name}.ps1, ${name}-cleanup.ps1"

/usr/local/bin/minikube-worker.sh "debian@${name}.local"

deadline=$(($(date +%s) + BOOT_SECONDS))
until [ "$(date +%s)" -ge "$deadline" ] || minikube kubectl -- get node "$name" > /dev/null 2>&1; do
  sleep "$POLL_SECONDS"
done
minikube kubectl -- annotate node "$name" "wake-on-lan.kaidotio.github.io/mac=${mac}" --overwrite
