---
paths:
  - "setup/windows/setup.ps1"
---

* Suggest `scp setup/windows/setup.ps1 <windows-user>@<windows-host>: && ssh -t <windows-user>@<windows-host> powershell -NoProfile -ExecutionPolicy Bypass -File setup.ps1 && ssh <windows-user>@<windows-host> powershell -NoProfile -NonInteractive -Command "Remove-Item setup.ps1"` to the user after changing `setup/windows/setup.ps1`, as `.claude/rules/files.md`'s `## Workflow` does with its `Suggest to user` column, rather than running it - the script stops at `Read-Host -AsSecureString` for a password only the user may type, and only a host that already ran it once from the console takes it this way, since the script is what installs the OpenSSH server
