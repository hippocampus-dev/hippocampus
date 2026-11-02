#Requires -RunAsAdministrator

$ErrorActionPreference = 'Stop'
$GitHubUser = 'kaidotio'
$ComputerName = 'hyperv-host'
# Windows PowerShell 5.1 reads a script without a BOM in the ANSI code page
$NetAdapterName = 'イーサネット'
$SwitchName = 'External'

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
Set-NetFirewallRule -Name OpenSSH-Server-In-TCP, Custom-RemoteDesktop-In-TCP, Custom-RemoteDesktop-In-UDP -Enabled True -Profile Any -RemoteAddress LocalSubnet

Set-NetAdapterPowerManagement -Name $NetAdapterName -WakeOnMagicPacket Enabled
powercfg.exe /hibernate off

if ((Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All -NoRestart).RestartNeeded) {
  'Restart Windows and run this script again'
  exit
}
if (-not (Get-VMSwitch -Name $SwitchName -ErrorAction SilentlyContinue)) {
  New-VMSwitch -Name $SwitchName -NetAdapterName $NetAdapterName -AllowManagementOS $true | Out-Null
}

winget install --id VirtualDrivers.Virtual-Display-Driver --exact --source winget --silent --accept-package-agreements --accept-source-agreements
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
