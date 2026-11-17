#Requires -RunAsAdministrator

$ErrorActionPreference = 'Stop'
$NvidiaDriverVersion = '597.30'
$RadeonDriverVersion = '26.10.1'
# https://www.amd.com/en/resources/support-articles/release-notes/RN-RAD-WIN-26-10-1.html
$RadeonDriverStoreVersion = '32.0.32015.7029'
$RadeonRdna2DriverStoreVersion = '32.0.21045.16010'

# https://pci-ids.ucw.cz/v2.2/pci.ids
$vendors = Get-CimInstance Win32_VideoController | Where-Object { $_.PNPDeviceID -like 'PCI\VEN_*' -and $_.Name -ne 'AMD Radeon(TM) Graphics' } | ForEach-Object { $_.PNPDeviceID.Substring(8, 4) } | Sort-Object -Unique
$Nvidia = $vendors -contains '10DE'
if ($Nvidia -eq ($vendors -contains '1002')) {
  throw "Expected either an NVIDIA or an AMD GPU, found PCI vendors: $($vendors -join ', ')"
}

powercfg.exe /hibernate off
powercfg.exe /change standby-timeout-ac 0
powercfg.exe /change monitor-timeout-ac 0
# Smart App Control blocks the unsigned confidant and the taurin installer that setup-windows.sh places: https://support.microsoft.com/en-us/topic/what-is-smart-app-control-285ea03d-fa88-4d56-882e-6698afdb7003
# https://learn.microsoft.com/en-us/windows/security/application-security/application-control/app-control-for-business/appcontrol
if ((Get-ItemProperty -Path HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy).VerifiedAndReputablePolicyState -ne 0) {
  Set-ItemProperty -Path HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy -Name VerifiedAndReputablePolicyState -Value 0
  CiTool.exe -r | Out-Null
}

if ($Nvidia) {
  $installed = (Get-CimInstance Win32_VideoController | Where-Object Name -like 'NVIDIA*').DriverVersion
  # https://www.nvidia.com/en-gb/drivers/drivers-faq/
  if (-not $installed -or [version](($installed -replace '\D', '') -replace '^\d*(\d{3})(\d{2})$', '$1.$2') -lt [version]$NvidiaDriverVersion) {
    winget install --id 7zip.7zip --exact --source winget --silent --accept-package-agreements --accept-source-agreements
    $package = Join-Path $env:TEMP "$NvidiaDriverVersion.exe"
    $directory = Join-Path $env:TEMP $NvidiaDriverVersion
    curl.exe -fsSL --retry 5 --retry-all-errors -o $package "https://us.download.nvidia.com/Windows/Quadro_Certified/$NvidiaDriverVersion/$NvidiaDriverVersion-quadro-rtx-desktop-notebook-win10-win11-64bit-international-dch-whql.exe"
    & "$env:ProgramFiles\7-Zip\7z.exe" x -y "-o$directory" $package | Out-Null
    # https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/windows.html
    $process = Start-Process -FilePath "$directory\setup.exe" -ArgumentList '-s', '-n', 'Display.Driver' -Wait -PassThru
    Remove-Item -Recurse $package, $directory
    if ($process.ExitCode -eq 1) {
      'Restart Windows and run this script again'
      exit
    }
    if ($process.ExitCode -ne 0) {
      throw "NVIDIA driver installer exited with $($process.ExitCode)"
    }
  }
} else {
  $controller = Get-CimInstance Win32_VideoController | Where-Object Name -like 'AMD Radeon RX*'
  $required = if ($controller.Name -match 'RX [56]\d{3}') { $RadeonRdna2DriverStoreVersion } else { $RadeonDriverStoreVersion }
  if (-not $controller -or [version]$controller.DriverVersion -lt [version]$required) {
    winget install --id 7zip.7zip --exact --source winget --silent --accept-package-agreements --accept-source-agreements
    $package = Join-Path $env:TEMP "$RadeonDriverVersion.exe"
    $directory = Join-Path $env:TEMP $RadeonDriverVersion
    # drivers.amd.com redirects a request without this Referer away from the file
    curl.exe -fsSL --retry 5 --retry-all-errors -H 'Referer: https://www.amd.com/' -o $package "https://drivers.amd.com/drivers/whql-amd-software-adrenalin-edition-$RadeonDriverVersion-win11-c.exe"
    & "$env:ProgramFiles\7-Zip\7z.exe" x -y "-o$directory" $package | Out-Null
    # https://drivers.amd.com/relnotes/command-line-installation.pdf
    $process = Start-Process -FilePath "$directory\Bin64\ATISetup.exe" -ArgumentList '-INSTALL', '-OUTPUT', 'screen' -NoNewWindow -Wait -PassThru
    Remove-Item -Recurse $package, $directory
    if ($process.ExitCode -ne 0) {
      throw "AMD driver installer exited with $($process.ExitCode)"
    }
  }
}

winget install --id Valve.Steam --exact --source winget --silent --accept-package-agreements --accept-source-agreements

# https://nsis.sourceforge.io/Docs/Chapter3.html
$process = Start-Process -FilePath "$PSScriptRoot\taurin-setup.exe" -ArgumentList '/S' -Wait -PassThru
if ($process.ExitCode -ne 0) {
  throw "taurin installer exited with $($process.ExitCode)"
}

# Kept where only an elevated process can write, since the task runs it with the highest privileges
$directory = Join-Path $env:ProgramFiles 'confidant'
New-Item -ItemType Directory -Force $directory | Out-Null
if (Get-ScheduledTask -TaskName confidant -ErrorAction SilentlyContinue) { Stop-ScheduledTask -TaskName confidant }
Get-Process -Name confidant -ErrorAction SilentlyContinue | Stop-Process -Force -PassThru | Wait-Process
# Move-Item would keep the ACL of the directory setup-windows.sh writes, while a copy inherits the destination's
Copy-Item -Force "$PSScriptRoot\confidant.exe" $directory
# VK_F13 (0x7C)
$action = New-ScheduledTaskAction -Execute (Join-Path $directory 'confidant.exe') -Argument '--confine-key 124 --confine-modifier control'
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
# SendInput cannot reach a window of a higher integrity level: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendinput
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Highest
# ExecutionTimeLimit defaults to 72 hours and Priority to 7, a below-normal priority class: https://learn.microsoft.com/en-us/windows/win32/taskschd/tasksettings-priority
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -Priority 4
Register-ScheduledTask -Force -TaskName confidant -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null
Start-ScheduledTask -TaskName confidant
