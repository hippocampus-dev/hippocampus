---
paths:
  - "setup/hyperv-host/setup.ps1"
  - "setup/hyperv-host/wsl.sh"
---

* Read the Windows SSH targets from `HYPERV_WORKERS` in `files/usr/local/lib/hyperv-host/lib.sh` and suggest `scp setup/hyperv-host/setup.ps1 setup/hyperv-host/wsl.sh <target>: && ssh -t <target> 'chcp 65001 >NUL && powershell -NoProfile -ExecutionPolicy Bypass -File setup.ps1' && ssh <target> 'chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command "Remove-Item setup.ps1, wsl.sh"'` for each target, replacing `<target>` with its registered value, after changing `setup/hyperv-host/setup.ps1` or `setup/hyperv-host/wsl.sh`, which `setup.ps1` runs from its own directory, as `.claude/rules/files.md`'s `## Workflow` does with its `Suggest to user` column, rather than running it - the script stops at `Read-Host -AsSecureString` for a password only the user may type, and only a host that already ran it once from the console takes it this way, since the script is what installs the OpenSSH server
