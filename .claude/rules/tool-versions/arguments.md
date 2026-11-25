---
paths:
  - "setup/windows/setup.ps1"
  - "files/usr/local/bin/setup-hyperv-host.sh"
  - "files/usr/local/bin/setup-steam-frame.sh"
  - "files/etc/systemd/user/confidant.service"
  - "files/etc/systemd/user/teleput-server.service"
---

* Commit a change passing an argument the release pinned in `files/home/kai/.tool-versions` lacks together with the bump of that line, or after a release carrying it where the line is `latest`, never before it - these hand their arguments to the pinned release rather than to the working tree, so the old build exits on the unknown argument, a Windows build with no console to show it (`windows_subsystem = "windows"` in `confidant/src/main.rs`) while its setup script still succeeds, and running the file for any other reason in between stops the program it starts
