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
# The R620 branch CUDA 13.5 needs, which the CUDA build of taurin is compiled with: https://docs.nvidia.com/cuda/cuda-toolkit-release-notes/index.html
$NvidiaDriverVersion = '621.24'
$CudaVersion = '13.5'
$RadeonDriverVersion = '26.10.1'
# https://www.amd.com/en/resources/support-articles/release-notes/RN-RAD-WIN-26-10-1.html
$RadeonDriverStoreVersion = '32.0.32015.7029'
$RadeonRdna2DriverStoreVersion = '32.0.21045.16010'
$GitHubRunnerCount = 4

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
# Smart App Control blocks the unsigned teleput, confidant and taurin that setup-hyperv-host.sh installs: https://support.microsoft.com/en-us/topic/what-is-smart-app-control-285ea03d-fa88-4d56-882e-6698afdb7003
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
foreach ($Port in 2222, 7860, 8188) {
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
# Sunshine fails to bind 48010 with WSAEACCES while it lies in the Linux ephemeral port range that mirrored networking reserves on the host when WSL starts
$action = New-ScheduledTaskAction -Execute powershell.exe -Argument "-NoProfile -NonInteractive -Command `"while ((Get-NetTCPConnection -State Listen -LocalPort 47984, 47989, 47990, 48010 -ErrorAction SilentlyContinue | Sort-Object LocalPort -Unique).Count -lt 4) { Start-Sleep -Seconds 1 }; wsl.exe --distribution $WslDistribution --user root --exec true`""
$task = Get-ScheduledTask -TaskName $WslDistribution -ErrorAction SilentlyContinue
# .wslconfig is read from %UserProfile%, so the task runs as this user rather than SYSTEM
if (-not $task -or $task.Actions[0].Execute -ne $action.Execute -or $task.Actions[0].Arguments -ne $action.Arguments) {
  $password = Read-Host -AsSecureString "Windows password for $env:USERNAME"
  Register-ScheduledTask -Force -TaskName $WslDistribution -Action $action -Trigger (New-ScheduledTaskTrigger -AtStartup) -User $env:USERNAME -Password ([Net.NetworkCredential]::new('', $password).Password) | Out-Null
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
  wsl.exe --shutdown
  Restart-Service -Name SunshineService
  Start-ScheduledTask -TaskName $WslDistribution
}
# https://github.com/LizardByte/Sunshine/blob/master/src_assets/windows/misc/firewall/add-firewall-rule.bat
Set-NetFirewallRule -DisplayName Sunshine -Profile Any -RemoteAddress LocalSubnet
$collectors = 'cpu,gpu,hyperv,logical_disk,memory,net,os,pagefile,system,tcp,textfile'
$service = Get-CimInstance Win32_Service -Filter "Name='windows_exporter'"
# https://github.com/prometheus-community/windows_exporter/blob/v0.32.1/installer/main.wxs
if ($service.PathName -notlike "* --collectors.enabled $collectors *") {
  if ($service) {
    winget uninstall --id Prometheus.WindowsExporter --exact --source winget --silent
  }
  # The portable installer in the same manifest registers no service: https://github.com/microsoft/winget-pkgs/blob/master/manifests/p/Prometheus/WindowsExporter/0.32.1/Prometheus.WindowsExporter.installer.yaml
  winget install --id Prometheus.WindowsExporter --exact --source winget --installer-type wix --silent --custom "ENABLED_COLLECTORS=$collectors EXTRA_FLAGS=--collector.hyperv.enabled=hypervisor_logical_processor" --accept-package-agreements --accept-source-agreements
}

# Prometheus v2.37 rejects the quoted label names that the /metrics endpoint of LibreHardwareMonitor writes: https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitor/Utilities/HttpServer.cs
$textfile = "$env:ProgramFiles\LibreHardwareMonitorTextfile"
if (Get-ScheduledTask -TaskName LibreHardwareMonitorTextfile -ErrorAction SilentlyContinue) { Stop-ScheduledTask -TaskName LibreHardwareMonitorTextfile }
# The SYSTEM task loads this library, so it stays under portablePackageMachineRoot, which only administrators can write: https://github.com/microsoft/winget-cli/blob/master/doc/Settings.md
$libreHardwareMonitor = "$env:ProgramFiles\WinGet\Packages\LibreHardwareMonitor.LibreHardwareMonitor_Microsoft.Winget.Source_8wekyb3d8bbwe"
winget install --id LibreHardwareMonitor.LibreHardwareMonitor --exact --source winget --scope machine --silent --accept-package-agreements --accept-source-agreements
# Add-Type fails on a library carrying the Mark of the Web: https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/discussions/1211
Get-ChildItem -Path $libreHardwareMonitor | Unblock-File
New-Item -ItemType Directory -Force -Path $textfile | Out-Null
Set-Content -Path "$textfile\textfile.ps1" -Encoding ascii -Value @'
param([string]$Directory)
Add-Type -Path "$Directory\LibreHardwareMonitorLib.dll"
$computer = [LibreHardwareMonitor.Hardware.Computer]@{ IsCpuEnabled = $true; IsMotherboardEnabled = $true }
$computer.Open()
$path = "$env:ProgramFiles\windows_exporter\textfile_inputs\libre_hardware_monitor.prom"
$units = @{ Temperature = 'celsius', 1; Clock = 'hertz', 1e6; Fan = 'rpm', 1; Voltage = 'volts', 1 }
while ($true) {
  $lines = foreach ($hardware in $computer.Hardware) {
    foreach ($item in @($hardware) + $hardware.SubHardware) {
      $item.Update()
      # A core clock holds no value until its counters advance: https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/blob/v0.9.6/LibreHardwareMonitorLib/Hardware/Cpu/Amd17Cpu.cs
      foreach ($sensor in $item.Sensors | Where-Object { $units.ContainsKey("$($_.SensorType)") -and $null -ne $_.Value }) {
        $unit, $factor = $units["$($sensor.SensorType)"]
        "lhm_$("$($hardware.HardwareType)_$($sensor.SensorType)".ToLower())_$unit{hardwareName=`"$($item.Name)`",sensorName=`"$($sensor.Name)`",sensorIndex=`"$($sensor.Index)`"} $($sensor.Value * $factor)"
      }
    }
  }
  Set-Content -Path "$path.tmp" -Encoding ascii -Value $lines
  Move-Item -Force -Path "$path.tmp" -Destination $path
  Start-Sleep -Seconds 15
}
'@
# A task triggered at startup runs no longer than 72 hours unless ExecutionTimeLimit says otherwise: https://learn.microsoft.com/en-us/windows/win32/taskschd/tasksettings-executiontimelimit
Register-ScheduledTask -Force -TaskName LibreHardwareMonitorTextfile -Action (New-ScheduledTaskAction -Execute powershell.exe -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$textfile\textfile.ps1`" -Directory `"$libreHardwareMonitor`"") -Trigger (New-ScheduledTaskTrigger -AtStartup) -User SYSTEM -Settings (New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero)) | Out-Null
Start-ScheduledTask -TaskName LibreHardwareMonitorTextfile

# The toolchain the windows-latest image carries for the Windows jobs of .github/workflows/20_taurin.yaml, 20_confidant.yaml and 20_teleput.yaml
foreach ($id in 'Git.Git', 'GitHub.cli', 'Kitware.CMake', 'LLVM.LLVM', 'Rustlang.Rustup') {
  winget install --id $id --exact --source winget --silent --accept-package-agreements --accept-source-agreements
}
winget install --id Microsoft.VisualStudio.2022.BuildTools --exact --source winget --silent --override '--wait --quiet --norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended' --accept-package-agreements --accept-source-agreements
$cuda = Join-Path $env:ProgramFiles "NVIDIA GPU Computing Toolkit\CUDA\v$CudaVersion"
if (-not (Test-Path -Path (Join-Path $cuda 'bin\nvcc.exe'))) {
  $package = Join-Path $env:TEMP "cuda_$($CudaVersion).0_windows_x86_64_network.exe"
  curl.exe -fsSL --retry 5 --retry-all-errors -o $package "https://developer.download.nvidia.com/compute/cuda/$CudaVersion.0/network_installers/cuda_$($CudaVersion).0_windows_x86_64_network.exe"
  # https://docs.nvidia.com/cuda/cuda-installation-guide-microsoft-windows/index.html
  $process = Start-Process -FilePath $package -ArgumentList '-s', "nvcc_$CudaVersion", "crt_$CudaVersion", "nvvm_$CudaVersion", "cccl_$CudaVersion", "cudart_$CudaVersion", "cublas_$CudaVersion", "cublas_dev_$CudaVersion", "curand_$CudaVersion", "curand_dev_$CudaVersion", "nvrtc_$CudaVersion", "nvrtc_dev_$CudaVersion", "visual_studio_integration_$CudaVersion" -Wait -PassThru
  Remove-Item $package
  if ($process.ExitCode -ne 0) {
    throw "CUDA installer exited with $($process.ExitCode)"
  }
}
if ($Nvidia) {
  # https://learn.microsoft.com/en-us/windows/win32/dlls/dynamic-link-library-search-order
  $environment = 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment'
  # [Environment]::SetEnvironmentVariable writes REG_SZ, which would leave the %SystemRoot% entries of Path expanded for good
  $path = (Get-Item -Path $environment).GetValue('Path', '', 'DoNotExpandEnvironmentNames')
  if (($path -split ';') -notcontains (Join-Path $cuda 'bin\x64')) {
    Set-ItemProperty -Path $environment -Name Path -Type ExpandString -Value "$path;$(Join-Path $cuda 'bin\x64')"
  }
}

$runner = "$env:LOCALAPPDATA\github-local-self-hosted-runner"
New-Item -ItemType Directory -Force -Path $runner | Out-Null
# setup-hyperv-host.sh logs gh in, which runner.ps1 requests registration tokens through
Set-Content -Path "$runner\runner.ps1" -Encoding ascii -Value @'
param([int]$Index)
$ErrorActionPreference = 'Stop'
$Repository = 'hippocampus-dev/hippocampus'
# The runner resolves shell: bash from PATH, where WSL's System32\bash.exe precedes Git for Windows: https://github.com/actions/runner/blob/main/src/Runner.Worker/Handlers/ScriptHandler.cs
$env:PATH = "$env:ProgramFiles\Git\bin;$env:PATH"
$version = (curl.exe -fsSL --retry 5 --retry-all-errors -H 'Accept: application/vnd.github+json' -H 'X-GitHub-Api-Version: 2022-11-28' https://api.github.com/repos/actions/runner/releases/latest | ConvertFrom-Json).tag_name.TrimStart('v')
$t = "$PSScriptRoot\runner$Index"
Remove-Item -Recurse -Force -ErrorAction Ignore $t
New-Item -ItemType Directory -Path $t | Out-Null
curl.exe -fsSL --retry 5 --retry-all-errors -o "$t\actions-runner-win-x64.zip" "https://github.com/actions/runner/releases/download/v$version/actions-runner-win-x64-$version.zip"
Expand-Archive -Path "$t\actions-runner-win-x64.zip" -DestinationPath $t
Remove-Item "$t\actions-runner-win-x64.zip"
Set-Location $t
# cl.exe stops with C1081 on a path past 260 characters, which the CUDA sources whisper-rs-sys copies under OUT_DIR reach beneath a deeper work directory: https://learn.microsoft.com/en-us/cpp/error-messages/compiler-errors-1/fatal-error-c1081
$work = "$env:SystemDrive\a$Index"
while ($true) {
  $token = & "$env:ProgramFiles\GitHub CLI\gh.exe" api -X POST "repos/$Repository/actions/runners/registration-token" --jq .token
  # gh api makes a single attempt, so a network that is not up yet at boot would otherwise end the task until the next one
  if ($LASTEXITCODE -ne 0) {
    Start-Sleep -Seconds 60
    continue
  }
  Remove-Item -Recurse -Force -ErrorAction Ignore $work
  .\config.cmd --url "https://github.com/$Repository" --token $token --name "local-runner-$env:COMPUTERNAME-$env:USERNAME-$Index" --work $work --labels 'self-hosted,Windows,X64,local' --ephemeral --replace --unattended
  if ($LASTEXITCODE -ne 0) {
    throw "config.cmd exited with $LASTEXITCODE"
  }
  .\run.cmd
}
'@
$windowsPassword = $null
foreach ($index in 1..$GitHubRunnerCount) {
  $name = "github-local-self-hosted-runner-$index"
  $action = New-ScheduledTaskAction -Execute powershell.exe -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$runner\runner.ps1`" -Index $index"
  $task = Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue
  if (-not $task -or $task.Actions[0].Execute -ne $action.Execute -or $task.Actions[0].Arguments -ne $action.Arguments) {
    if (-not $windowsPassword) {
      $windowsPassword = Read-Host -AsSecureString "Windows password for $env:USERNAME"
    }
    Register-ScheduledTask -Force -TaskName $name -Action $action -Trigger (New-ScheduledTaskTrigger -AtStartup) -User $env:USERNAME -Password ([Net.NetworkCredential]::new('', $windowsPassword).Password) -Settings (New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero)) | Out-Null
  }
  Start-ScheduledTask -TaskName $name
}
