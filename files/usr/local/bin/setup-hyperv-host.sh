#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly REPOSITORY=hippocampus-dev/hippocampus
readonly TASK_NAME=teleput

usage() {
  cat <<EOS
Usage:
   setup-hyperv-host.sh [<node>|<windows-user>@<windows-host>]

Set up every configured Hyper-V worker, or only the selected node or Windows SSH target
Install and pair teleput only for workers with a configured teleput edge, registering the task ${TASK_NAME} at sign-in with the highest privileges
Install confidant on every worker, registering the task confidant at sign-in with the highest privileges
Install taurin (CUDA build on NVIDIA GPUs) on every worker at the version ~/.tool-versions names, or from the latest ${REPOSITORY} release for latest, starting it in the signed-in session
Log gh on every worker in with GITHUB_TOKEN, which the GitHub Actions runners setup.ps1 registers use to register themselves on ${REPOSITORY}
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

if [ -z "$GITHUB_TOKEN" ]; then
  echo "Please declare required environment variables: GITHUB_TOKEN" 1>&2
  exit 1
fi

source /usr/local/lib/hyperv-host/lib.sh
select_hyperv_workers "${args[0]:-}"

for worker in "${hyperv_workers[@]}"; do
  (
    read -r worker_target target _ teleput_edge <<< "$worker"
    worker_host="${worker_target#*@}"
    vm_name="${worker_host%.local}"
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
    trap 'rm -rf "$work"' EXIT

    if [ -n "$teleput_edge" ]; then
      server="$(uname -n).local"
      server_fingerprint=$(teleput fingerprint)

      echo "=== teleput: ${version} for windows"
      gh release download "v${version}" --repo "$REPOSITORY" --pattern "teleput_${version}_windows_amd64.exe" --output "${work}/teleput.exe"

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

    echo "=== confidant: ${version} for windows"
    gh release download "v${version}" --repo "$REPOSITORY" --pattern "confidant_${version}_windows_amd64.exe" --output "${work}/confidant.exe"

    printf '\xef\xbb\xbf' > "${work}/confidant-install.ps1"
    cat <<'EOS' >> "${work}/confidant-install.ps1"
$ErrorActionPreference = 'Stop'
# Kept where only an elevated process can write, since the task runs it with the highest privileges
$directory = Join-Path $env:ProgramFiles 'confidant'
New-Item -ItemType Directory -Force $directory | Out-Null
if (Get-ScheduledTask -TaskName confidant -ErrorAction SilentlyContinue) { Stop-ScheduledTask -TaskName confidant }
Get-Process -Name confidant -ErrorAction SilentlyContinue | Stop-Process -Force -PassThru | Wait-Process
# Move-Item would keep the ACL of the user's home, while a copy inherits the directory's
Copy-Item -Force "$HOME\confidant.exe" $directory
Remove-Item "$HOME\confidant.exe"
# VK_F13 (0x7C)
$action = New-ScheduledTaskAction -Execute (Join-Path $directory 'confidant.exe') -Argument '--linear --confine-key 124 --confine-modifier control'
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
# SendInput cannot reach a window of a higher integrity level: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendinput
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Highest
# ExecutionTimeLimit defaults to 72 hours and Priority to 7, a below-normal priority class: https://learn.microsoft.com/en-us/windows/win32/taskschd/tasksettings-priority
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -Priority 4
Register-ScheduledTask -Force -TaskName confidant -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null
# An Interactive task runs only while the user is signed in, and the AtLogOn trigger starts it otherwise: https://learn.microsoft.com/en-us/windows/win32/taskschd/principal-logontype
if (Get-Process -Name explorer -ErrorAction SilentlyContinue) { Start-ScheduledTask -TaskName confidant }
EOS

    scp "${work}/confidant.exe" "${work}/confidant-install.ps1" "${target}:"
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File confidant-install.ps1"
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Remove-Item confidant-install.ps1\""

    # https://pci-ids.ucw.cz/v2.2/pci.ids
    vendors=$(ssh "$target" 'chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command "$ErrorActionPreference = '\''Stop'\''; Get-CimInstance Win32_VideoController | Where-Object { $_.PNPDeviceID -like '\''PCI\VEN_*'\'' -and $_.Name -ne '\''AMD Radeon(TM) Graphics'\'' } | ForEach-Object { $_.PNPDeviceID.Substring(8, 4) } | Sort-Object -Unique"' | tr -d '\r')
    case "$(awk '/^(10DE|1002)$/' <<< "$vendors")" in
      10DE)
        taurin_pattern="taurin_${taurin_version}_x64_cuda-setup.exe"
        ;;
      1002)
        taurin_pattern="taurin_${taurin_version}_x64-setup.exe"
        ;;
      *)
        echo "Expected either an NVIDIA or an AMD GPU, found PCI vendors: ${vendors//$'\n'/, }" 1>&2
        exit 1
        ;;
    esac
    echo "=== taurin: ${taurin_pattern}"
    gh release download "v${taurin_version}" --repo "$REPOSITORY" --pattern "$taurin_pattern" --output "${work}/taurin-setup.exe"

    printf '\xef\xbb\xbf' > "${work}/taurin-install.ps1"
    cat <<'EOS' >> "${work}/taurin-install.ps1"
$ErrorActionPreference = 'Stop'
Get-Process -Name taurin -ErrorAction SilentlyContinue | Stop-Process -Force -PassThru | Wait-Process
# The NSIS bundle installs per user under %LOCALAPPDATA%\taurin, and /S runs it without a window: https://v2.tauri.app/distribute/windows-installer/
$installer = Start-Process -Wait -PassThru -FilePath "$HOME\taurin-setup.exe" -ArgumentList '/S'
if ($installer.ExitCode -ne 0) { throw "taurin-setup.exe exited with $($installer.ExitCode)" }
Remove-Item "$HOME\taurin-setup.exe"
# A process started over SSH has no desktop, so the task starts it in the signed-in session instead; the Auto Start setting covers sign-in
$action = New-ScheduledTaskAction -Execute (Join-Path $env:LOCALAPPDATA 'taurin\taurin.exe')
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -Priority 4
Register-ScheduledTask -Force -TaskName taurin -Action $action -Principal $principal -Settings $settings | Out-Null
if (Get-Process -Name explorer -ErrorAction SilentlyContinue) {
  Start-ScheduledTask -TaskName taurin
}
EOS

    scp "${work}/taurin-setup.exe" "${work}/taurin-install.ps1" "${target}:"
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File taurin-install.ps1"
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Remove-Item taurin-install.ps1\""

    echo "=== github-local-self-hosted-runner: gh auth login"
    printf '%s' "$GITHUB_TOKEN" | ssh "$target" 'chcp 65001 >NUL && "%ProgramFiles%\GitHub CLI\gh.exe" auth login --with-token'
    ssh "$target" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"Get-ScheduledTask -TaskName 'github-local-self-hosted-runner-*' -ErrorAction SilentlyContinue | Start-ScheduledTask\""

    minikube-add-hyperv-worker.sh "$vm_name"
  )
done
