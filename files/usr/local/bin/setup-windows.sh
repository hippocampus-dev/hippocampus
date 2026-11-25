#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly REPOSITORY=hippocampus-dev/hippocampus
# C: of the Windows that dual-boots with this host
readonly WINDOWS_PARTITION=/dev/disk/by-uuid/B09080C490809290

version=$(awk '$1 == "hippocampus" {print $2}' ~/.tool-versions)
if [ "$version" = "latest" ]; then
  tag=$(gh release view --repo "$REPOSITORY" --json tagName --jq '.tagName')
  version="${tag#v}"
fi
taurin_version=$(awk '$1 == "taurin" {print $2}' ~/.tool-versions)
if [ "$taurin_version" = "latest" ]; then
  tag=$(gh release view --repo "$REPOSITORY" --json tagName --jq '.tagName')
  taurin_version="${tag#v}"
fi

work=$(mktemp -d)
windows=$(mktemp -d)
cleanup() {
  if mountpoint -q "$windows"; then
    sudo umount "$windows"
  fi
  # rmdir rather than rm -rf, which would delete the files of Windows were the partition still mounted
  rmdir "$windows"
  rm -rf "$work"
}
trap cleanup EXIT

echo "=== confidant: ${version} for windows"
gh release download "v${version}" --repo "$REPOSITORY" --pattern "confidant_${version}_windows_amd64.exe" --output "${work}/confidant.exe"

echo "=== taurin: ${taurin_version} for windows"
gh release download "v${taurin_version}" --repo "$REPOSITORY" --pattern "taurin_${taurin_version}_x64-setup.exe" --output "${work}/taurin-setup.exe"
gh release download "v${taurin_version}" --repo "$REPOSITORY" --pattern "taurin_${taurin_version}_x64_cuda-setup.exe" --output "${work}/taurin-cuda-setup.exe"

echo "=== C:\\hippocampus: setup.ps1 and the downloads"
# ntfs3 refuses read-write only for a dirty volume or an unreplayed log, not for the hibernation ntfs-3g checks: https://github.com/torvalds/linux/blob/master/fs/ntfs3/super.c
# https://man.archlinux.org/man/ntfs-3g.8
sudo mount -t ntfs3 -o ro "$WINDOWS_PARTITION" "$windows"
header=
if sudo test -f "${windows}/hiberfil.sys"; then
  header=$(sudo head -c 4 "${windows}/hiberfil.sys" | tr -d '\0')
fi
sudo umount "$windows"
if [ "${header,,}" = "hibr" ]; then
  echo "Windows is hibernated or was shut down with Fast Startup, so boot it and restart into Arch Linux rather than shutting it down, then run this again" 1>&2
  exit 1
fi
sudo mount -t ntfs3 "$WINDOWS_PARTITION" "$windows"
sudo mkdir -p "${windows}/hippocampus"
sudo cp /opt/hippocampus/setup/windows/setup.ps1 "${work}/confidant.exe" "${work}/taurin-setup.exe" "${work}/taurin-cuda-setup.exe" "${windows}/hippocampus/"
