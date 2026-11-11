#Requires -RunAsAdministrator

param(
  [string]$ComputerName = $env:COMPUTERNAME,
  [string]$SwitchName = 'External'
)

$ErrorActionPreference = 'Stop'
$GitHubUser = 'kaidotio'
# Windows PowerShell 5.1 reads a script without a BOM in the ANSI code page
$NetAdapterName = 'イーサネット'
$WslDistribution = 'Ubuntu-24.04'
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

if ($env:COMPUTERNAME -ne $ComputerName) {
  Rename-Computer -NewName $ComputerName
}

Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0 | Out-Null
Set-Service -Name sshd -StartupType Automatic
Start-Service -Name sshd
# https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_keymanagement#administrative-user
$keys = "$env:ProgramData\ssh\administrators_authorized_keys"
Set-Content -Path $keys -Encoding ascii -Value (Invoke-RestMethod "https://github.com/$GitHubUser.keys")
icacls.exe $keys /inheritance:r /grant '*S-1-5-32-544:F' /grant '*S-1-5-18:F' | Out-Null

Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name fDenyTSConnections -Value 0
# The Windows 11 25H2 feature update disabled the built-in RemoteDesktop-UserMode-In-TCP rule
foreach ($Protocol in 'TCP', 'UDP') {
  if (-not (Get-NetFirewallRule -Name "Custom-RemoteDesktop-In-$Protocol" -ErrorAction SilentlyContinue)) {
    New-NetFirewallRule -Name "Custom-RemoteDesktop-In-$Protocol" -DisplayName "Custom Remote Desktop ($Protocol-In)" -Protocol $Protocol -LocalPort 3389 | Out-Null
  }
}
if (-not (Get-NetFirewallRule -Name Custom-WindowsExporter-In-TCP -ErrorAction SilentlyContinue)) {
  New-NetFirewallRule -Name Custom-WindowsExporter-In-TCP -DisplayName 'Custom windows_exporter (TCP-In)' -Protocol TCP -LocalPort 9182 | Out-Null
}
Set-NetFirewallRule -Name OpenSSH-Server-In-TCP, Custom-RemoteDesktop-In-TCP, Custom-RemoteDesktop-In-UDP, Custom-WindowsExporter-In-TCP -Enabled True -Profile Any -RemoteAddress LocalSubnet

Set-NetAdapterPowerManagement -Name $NetAdapterName -WakeOnMagicPacket Enabled
powercfg.exe /hibernate off
powercfg.exe /change standby-timeout-ac 0
powercfg.exe /change monitor-timeout-ac 0
# Smart App Control blocks the unsigned teleput and taurin that setup-hyperv-host.sh installs: https://support.microsoft.com/en-us/topic/what-is-smart-app-control-285ea03d-fa88-4d56-882e-6698afdb7003
# https://learn.microsoft.com/en-us/windows/security/application-security/application-control/app-control-for-business/appcontrol
if ((Get-ItemProperty -Path HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy).VerifiedAndReputablePolicyState -ne 0) {
  Set-ItemProperty -Path HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy -Name VerifiedAndReputablePolicyState -Value 0
  CiTool.exe -r | Out-Null
}

if ((Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All, VirtualMachinePlatform -NoRestart).RestartNeeded) {
  'Restart Windows and run this script again'
  exit
}
if (-not (Get-VMSwitch -Name $SwitchName -ErrorAction SilentlyContinue)) {
  New-VMSwitch -Name $SwitchName -NetAdapterName $NetAdapterName -AllowManagementOS $true | Out-Null
}

if ($Nvidia) {
  # nvidia-smi.exe prints "Failed to initialize NVML: Unknown Error" over OpenSSH
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

# https://learn.microsoft.com/en-us/windows/wsl/wsl-config#wslconfig
$wslconfig = @'
[wsl2]
networkingMode=mirrored

[general]
instanceIdleTimeout=-1
'@
if ((Get-Content -Raw -Path "$HOME\.wslconfig" -ErrorAction SilentlyContinue) -ne $wslconfig) {
  Set-Content -Path "$HOME\.wslconfig" -Encoding ascii -NoNewline -Value $wslconfig
  wsl.exe --shutdown
}
# https://learn.microsoft.com/en-us/windows/wsl/networking#mirrored-mode-networking
foreach ($Port in 2222, 7860) {
  if (-not (Get-NetFirewallHyperVRule -Name "WSL-In-TCP-$Port" -ErrorAction SilentlyContinue)) {
    New-NetFirewallHyperVRule -Name "WSL-In-TCP-$Port" -DisplayName "WSL (TCP-In $Port)" -Direction Inbound -VMCreatorId '{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}' -Protocol TCP -LocalPorts $Port -RemoteAddresses 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fe80::/10, fc00::/7 | Out-Null
  }
}

wsl.exe --distribution $WslDistribution --user root --exec true | Out-Null
if ($LASTEXITCODE -ne 0) {
  wsl.exe --install $WslDistribution --no-launch
}
wsl.exe --distribution $WslDistribution --user root --cd $PSScriptRoot --exec bash wsl.sh $(if ($Nvidia) { '--nvidia' } else { '--radeon' })
if ($LASTEXITCODE -ne 0) {
  throw "Provisioning $WslDistribution exited with $LASTEXITCODE"
}
# .wslconfig is read from %UserProfile%, so the task runs as this user rather than SYSTEM
if (-not (Get-ScheduledTask -TaskName $WslDistribution -ErrorAction SilentlyContinue)) {
  $password = Read-Host -AsSecureString "Windows password for $env:USERNAME"
  Register-ScheduledTask -TaskName $WslDistribution -Action (New-ScheduledTaskAction -Execute wsl.exe -Argument "--distribution $WslDistribution --user root --exec true") -Trigger (New-ScheduledTaskTrigger -AtStartup) -User $env:USERNAME -Password ([Net.NetworkCredential]::new('', $password).Password) | Out-Null
}

# Sunshine streams an existing display, which a host without a monitor does not have: https://www.nite07.com/en/posts/extended-screen/
winget install --id VirtualDrivers.Virtual-Display-Driver --exact --source winget --silent --accept-package-agreements --accept-source-agreements
# https://github.com/VirtualDrivers/Virtual-Display-Driver/blob/25.7.23/Virtual%20Display%20Driver%20(HDR)/MttVDD/Driver.cpp
$vddSettings = @'
<?xml version="1.0" encoding="utf-8"?>
<vdd_settings>
  <monitors>
    <count>1</count>
  </monitors>
  <gpu>
    <friendlyname>default</friendlyname>
  </gpu>
  <resolutions>
    <resolution>
      <width>3840</width>
      <height>2160</height>
      <refresh_rate>60</refresh_rate>
    </resolution>
  </resolutions>
  <options>
    <CustomEdid>false</CustomEdid>
    <PreventSpoof>false</PreventSpoof>
    <EdidCeaOverride>false</EdidCeaOverride>
    <HardwareCursor>true</HardwareCursor>
    <SDR10bit>false</SDR10bit>
    <HDRPlus>false</HDRPlus>
    <logging>false</logging>
    <debuglogging>false</debuglogging>
  </options>
</vdd_settings>
'@
if ((Get-Content -Raw -Path C:\VirtualDisplayDriver\vdd_settings.xml -ErrorAction SilentlyContinue) -ne $vddSettings) {
  New-Item -ItemType Directory -Force -Path C:\VirtualDisplayDriver | Out-Null
  Set-Content -Path C:\VirtualDisplayDriver\vdd_settings.xml -Encoding ascii -NoNewline -Value $vddSettings
  pnputil.exe /restart-device /deviceid 'Root\MttVDD' | Out-Null
}
Set-ItemProperty -Path 'HKCU:\Control Panel\Desktop' -Name Win8DpiScaling -Value 1
Set-ItemProperty -Path 'HKCU:\Control Panel\Desktop' -Name LogPixels -Value 144
winget install --id Valve.Steam --exact --source winget --silent --accept-package-agreements --accept-source-agreements
if (-not (Get-Service -Name SunshineService -ErrorAction SilentlyContinue)) {
  winget install --id LizardByte.Sunshine --exact --source winget --silent --accept-package-agreements --accept-source-agreements
  # savePassword lets anyone set the admin credentials until they exist: https://github.com/LizardByte/Sunshine/blob/master/src/confighttp.cpp
  $password = Read-Host -AsSecureString "Sunshine password for $env:USERNAME"
  & "$env:ProgramFiles\Sunshine\sunshine.exe" --creds $env:USERNAME ([Net.NetworkCredential]::new('', $password).Password)
  Restart-Service -Name SunshineService
}
# https://github.com/LizardByte/Sunshine/blob/master/src_assets/windows/misc/firewall/add-firewall-rule.bat
Set-NetFirewallRule -DisplayName Sunshine -Profile Any -RemoteAddress LocalSubnet
if (-not (Get-Service -Name windows_exporter -ErrorAction SilentlyContinue)) {
  # The portable installer in the same manifest registers no service: https://github.com/microsoft/winget-pkgs/blob/master/manifests/p/Prometheus/WindowsExporter/0.32.1/Prometheus.WindowsExporter.installer.yaml
  winget install --id Prometheus.WindowsExporter --exact --source winget --installer-type wix --silent --custom 'ENABLED_COLLECTORS=cpu,gpu,hyperv,logical_disk,memory,net,os,pagefile,system,tcp EXTRA_FLAGS=--collector.hyperv.enabled=hypervisor_logical_processor' --accept-package-agreements --accept-source-agreements
}
