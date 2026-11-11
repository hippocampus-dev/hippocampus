---
paths:
  - "setup/windows/setup.ps1"
  - "files/usr/local/bin/setup-windows.sh"
---

* Suggest running `setup-windows.sh` on the Arch Linux host and then `powershell -ExecutionPolicy Bypass -File C:\hippocampus\setup.ps1` at an elevated PowerShell after booting Windows, after changing `setup/windows/setup.ps1` or `files/usr/local/bin/setup-windows.sh`, rather than running either - `setup-windows.sh` copies `setup.ps1` onto the Windows partition rather than linking it, so an edit reaches `C:\hippocampus\` only once it runs again, and `setup.ps1` runs only on the Windows that dual-boots with this host
