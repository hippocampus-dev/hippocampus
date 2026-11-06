#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly GITHUB_USER=kaidotio
readonly TASK_NAME=teleput

usage() {
  cat <<EOS
Usage:
   setup-hyperv-host.sh [<node>|<windows-user>@<windows-host>]

Set up every configured Hyper-V worker, or only the selected node or Windows SSH target
Install and pair teleput only for workers with a configured teleput edge, registering the task ${TASK_NAME} at sign-in with the highest privileges
Create each worker with minikube-add-hyperv-worker.sh unless its node already exists
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

for worker in "${hyperv_workers[@]}"; do
  (
    read -r worker_target target _ teleput_edge <<< "$worker"
    worker_host="${worker_target#*@}"
    vm_name="${worker_host%.local}"
    if [ -n "$teleput_edge" ]; then
      version=$(awk '$1 == "hippocampus" {print $2}' ~/.tool-versions)
      server="$(uname -n).local"
      server_fingerprint=$(teleput fingerprint)

      work=$(mktemp -d)
      trap 'rm -rf "$work"' EXIT

      echo "=== teleput: ${version} for windows"
      gh release download "v${version}" --repo "${GITHUB_USER}/hippocampus" --pattern "teleput_${version}_windows_amd64.exe" --output "${work}/teleput.exe"

      # Windows PowerShell 5.1 reads a script without a BOM in the ANSI code page
      printf '\xef\xbb\xbf' > "${work}/teleput-client.ps1"
      cat <<EOS >> "${work}/teleput-client.ps1"
while (\$true) {
  & "\$PSScriptRoot\\teleput.exe" client ${server}:24801
  Start-Sleep -Seconds 5
}
EOS

      printf '\xef\xbb\xbf' > "${work}/teleput-install.ps1"
      cat <<EOS >> "${work}/teleput-install.ps1"
\$ErrorActionPreference = 'Stop'
# Kept where only an elevated process can write, since the task runs it with the highest privileges
\$directory = Join-Path \$env:ProgramFiles 'teleput'
New-Item -ItemType Directory -Force \$directory | Out-Null
if (Get-ScheduledTask -TaskName '${TASK_NAME}' -ErrorAction SilentlyContinue) { Stop-ScheduledTask -TaskName '${TASK_NAME}' }
Get-Process -Name teleput -ErrorAction SilentlyContinue | Stop-Process -Force -PassThru | Wait-Process
# Move-Item would keep the ACL of the user's home, while a copy inherits the directory's
Copy-Item -Force "\$HOME\\teleput.exe", "\$HOME\\teleput-client.ps1" \$directory
Remove-Item "\$HOME\\teleput.exe", "\$HOME\\teleput-client.ps1"
New-Item -ItemType Directory -Force (Join-Path \$env:APPDATA 'teleput') | Out-Null
Set-Content -Path (Join-Path \$env:APPDATA 'teleput\\peer.sha256') -Encoding ascii -NoNewline -Value '${server_fingerprint}'
\$action = New-ScheduledTaskAction -Execute powershell.exe -Argument ('-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + \$directory + '\\teleput-client.ps1"')
\$trigger = New-ScheduledTaskTrigger -AtLogOn -User \$env:USERNAME
# SendInput cannot reach a window of a higher integrity level: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendinput
\$principal = New-ScheduledTaskPrincipal -UserId \$env:USERNAME -LogonType Interactive -RunLevel Highest
# ExecutionTimeLimit defaults to 72 hours and Priority to 7, a below-normal priority class: https://learn.microsoft.com/en-us/windows/win32/taskschd/tasksettings-priority
\$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -Priority 4
Register-ScheduledTask -Force -TaskName '${TASK_NAME}' -Action \$action -Trigger \$trigger -Principal \$principal -Settings \$settings | Out-Null
EOS

      scp "${work}/teleput.exe" "${work}/teleput-client.ps1" "${work}/teleput-install.ps1" "${target}:"
      ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File teleput-install.ps1"
      ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Remove-Item teleput-install.ps1\""

      echo "=== peer.${teleput_edge}.sha256: pairing over SSH"
      # Runs before the task starts, since whichever teleput runs first generates the certificate without a lock (load_or_generate in packages/teleput/src/identity.rs)
      client_fingerprint=$(ssh "$target" 'chcp 65001 >NUL && "%ProgramFiles%\teleput\teleput.exe" fingerprint' | tr -d '\r')
      (
        umask 077
        mkdir -p "${XDG_CONFIG_HOME:-${HOME}/.config}/teleput"
        printf '%s\n' "$client_fingerprint" > "${XDG_CONFIG_HOME:-${HOME}/.config}/teleput/peer.${teleput_edge}.sha256"
      )
      # teleput server reads peer.<edge>.sha256 only at startup (packages/teleput/src/server.rs)
      systemctl --user daemon-reload
      systemctl --user try-restart teleput-server.service
      # An Interactive task runs only while the user is signed in, and the AtLogOn trigger starts it otherwise: https://learn.microsoft.com/en-us/windows/win32/taskschd/principal-logontype
      ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"if (Get-Process -Name explorer -ErrorAction SilentlyContinue) { Start-ScheduledTask -TaskName ${TASK_NAME} }\""
    fi

    minikube-add-hyperv-worker.sh "$vm_name"
  )
done
