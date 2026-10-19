#Requires -RunAsAdministrator
param(
  [Parameter(Mandatory)][string]$NetAdapterName,
  [Parameter(Mandatory)][string]$SwitchName
)

$ErrorActionPreference = 'Stop'
$GitHubUser = 'kaidotio'

Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0 | Out-Null
Set-Service -Name sshd -StartupType Automatic
Start-Service -Name sshd
# https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_keymanagement#administrative-user
$keys = "$env:ProgramData\ssh\administrators_authorized_keys"
Set-Content -Path $keys -Encoding ascii -Value (Invoke-RestMethod "https://github.com/$GitHubUser.keys")
icacls.exe $keys /inheritance:r /grant '*S-1-5-32-544:F' /grant '*S-1-5-18:F' | Out-Null

Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name fDenyTSConnections -Value 0
Set-NetFirewallRule -Name OpenSSH-Server-In-TCP, RemoteDesktop-UserMode-In-TCP, RemoteDesktop-UserMode-In-UDP -Enabled True -Profile Any

Set-NetAdapterPowerManagement -Name $NetAdapterName -WakeOnMagicPacket Enabled
powercfg.exe /hibernate off

if ((Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V-All -NoRestart).RestartNeeded) {
  'Restart Windows and run this script again'
  exit
}
if (-not (Get-VMSwitch -Name $SwitchName -ErrorAction SilentlyContinue)) {
  New-VMSwitch -Name $SwitchName -NetAdapterName $NetAdapterName -AllowManagementOS $true | Out-Null
}
