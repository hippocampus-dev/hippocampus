# Installation

<!-- TOC -->
* [Installation](#installation)
  * [UEFI](#uefi)
  * [Windows](#windows)
  * [Arch Linux](#arch-linux)
    * [mozc](#mozc)
    * [Wireshark](#wireshark)
    * [Google Chrome](#google-chrome)
    * [IntelliJ](#intellij)
    * [How to set up a VM for Windows 11](#how-to-set-up-a-vm-for-windows-11)
<!-- TOC -->

## UEFI

1. AI Tweaker | Precision Boost Overdrive | Precision Boost Overdrive | AMD ECO Mode
2. AI Tweaker | Ai Overclock Tuner | DOCP I
3. Advanced | Onboard Devices Configuration | LED lighting | Stealth Mode
4. Tweak FAN settings to Silent
5. Tweak Boot Option Priorities to prioritize Linux

## Windows

1. Install Windows(Disk: 500GB)
2. Disable Fast Startup

## Arch Linux

```sh
$ iwctl station "$WIRELESS_NIC" connect "$SSID" --passphrase "$PASSWORD"
$ gdisk /dev/nvme0n1
# Use the rest of the disk for LVM
$ LVM_PARTITION=/dev/nvme0n1p5
$ pvcreate $LVM_PARTITION
$ vgcreate vg0 $LVM_PARTITION
$ lvcreate -L 1000G -T vg0/thinpool
$ lvcreate -V 1000G vg0/thinpool -n root
$ cryptsetup luksFormat -c aes-xts-plain64 -s 512 /dev/mapper/vg0-root
$ cryptsetup open /dev/mapper/vg0-root root
$ mkfs.ext4 /dev/mapper/root

$ mount /dev/mapper/root /mnt
$ mkdir /mnt/boot
$ mount /dev/nvme0n1p1 /mnt/boot
$ pacstrap /mnt base base-devel
$ genfstab -U /mnt >> /mnt/etc/fstab
$ lvcreate -L 100G vg0 -n swap
$ mkswap /dev/vg0/swap
$ echo "/dev/vg0/swap none swap defaults 0 0" >> /mnt/etc/fstab
$ lvcreate -V 100G vg0/thinpool -n nfs
$ mkfs.xfs /dev/vg0/nfs
$ echo "/dev/vg0/nfs /srv/nfs xfs defaults 0 0" >> /mnt/etc/fstab
$ arch-chroot /mnt

$ pacman -S vim linux amd-ucode linux-firmware mkinitcpio lvm2 iwd nvidia git
$ ln -sf /usr/share/zoneinfo/Asia/Tokyo /etc/localtime
$ hwclock --systohc --utc
$ vim /etc/locale.gen # comment out en_US.UTF-8,ja_JP.UTF-8
$ locale-gen
$ echo LANG=en_US.UTF-8 > /etc/locale.conf
$ echo arch > /etc/hostname
$ dd bs=512 count=4 if=/dev/urandom of=/etc/luks_keyfile
$ chmod 400 /etc/luks_keyfile
$ cryptsetup luksAddKey /dev/mapper/vg0-root /etc/luks_keyfile
$ vim /etc/lvm/lvm.conf # add `--auto-repair` to `thin_check_options`
$ vim /etc/mkinitcpio.conf # add lvm2, resume and encrypt to HOOKS, remove kms from HOOKS, add /etc/luks_keyfile to FILES
$ vim /etc/mkinitcpio.d/linux.preset # remove `fallback` from `PRESETS`
$ vim /etc/pacman.conf # uncomment `core-testing`, `extra-testing`, `multilib-testing`, `multilib`
$ mkinitcpio -p linux
$ passwd

$ bootctl --path=/boot install
$ bootctl update
$ vim /boot/loader/loader.conf
timeout 10
default arch
$ vim /boot/loader/entries/arch.conf
title arch
linux /vmlinuz-linux
initrd /amd-ucode.img
initrd /initramfs-linux.img
options cryptdevice=/dev/mapper/vg0-root:root cryptkey=rootfs:/etc/luks_keyfile resume=/dev/vg0/swap root=/dev/mapper/root rw lsm=capability,landlock,lockdown,yama,bpf,apparmor

$ git clone --recursive https://github.com/kaidotio/hippocampus /opt/hippocampus
$ exit
$ reboot

$ cd /opt/hippocampus
$ bash setup.sh
```

### mozc

1. Open `fcitx5-configtool`
2. Uncheck `Addons | Mozc | Configure | Configuration Tool | Configure | Suggest | Use input history`

### Wireshark

1. `Edit | Preferences | Appearance | Protocols | TLS | (Pre)-Master-Secret log filename` to `/tmp/sslkey.log`

### Google Chrome

1. Login
2. Tweak `Font size`
3. Install `extension`

### IntelliJ

1. Login
2. Install Plugins
3. Change `Keymap` to `Mine`
4. Change `Appearance & Behavior | Appearance | Zoom` to `150%`
5. Change `Tools | Terminal | Terminal Engine` to `Reworked 2025`
6. Change `Tools | AI Assistant | Features | Natural Language | Receive AI Assistant chat responses in a custom language` to `Japanese`
7. Check `Tools | AI Assistant | Features | Allow attaching database schemas to AI Assistant chat`
8. Download `Editor | General | Inline Completion | Enable local Full Line completion suggestions`
9. Check `Editor | General | Inline Completion | Enable cloud completion suggestions | SQL`
10. Check `Editor | General | Inline Completion | Enable cloud completion suggestions | Universal completion`
11. Change `Editor | General | Inline Completion | Enable cloud completion suggestions | Completion policy` to `Creative`
12. Check `Editor | General | Inline Completion | Synchronize inline and popup completions`
13. Change `Editor | General | On Save | Remove trailing spaces on` to `All lines`
14. Uncheck `Editor | General | On Save | Keep trailing spaces on caret line`
15. Check `Editor | General | On Save | Remove trailing blank lines at the end of saved files`
16. Check `Editor | General | On Save | Ensure every saved file ends with a line break`
17. Change `Editor | General | Soft Wraps | Soft-wraps these files` to `*`
18. Check `Editor | General | Appearance | Show whitespaces`
19. Change `Editor | Font | Size` to `16.0`
20. Change `Editor | Code Style | Go | Imports | Sorting type` to `goimports`
21. Change `Editor | Code Style | Python | Tabs and Indents | Continuation Indent` to `4`
22. Uncheck `Editor | Code Style | Python | Other | Use continuation indent for | Method declaration parameters`
23. Change `Editor | Natural Language | Grazie Pro | Text Completion | Completion` to `Always`
24. Setup `Tools | Actions on Save | Reformat code` on `Go`, `Python`, `JavaScript`, `TypeScript`, `Rust`
25. Setup `Tools | Actions on Save | Optimize imports` on `Go`, `Python`, `JavaScript`, `TypeScript`, `Rust`
26. Setup `Languages & Framework | Go | GOROOT`
27. Check `Languages & Framework | Go | Go Modules | Enable Go modules integration`
28. Change `Languages & Framework | Rust | External Linters | External linter` to `Clippy`
29. Check `Languages & Framework | Rust | External Linters | Run external linter on the fly`
30. Add `/opt/hippocampus/.idea/crd/crd.yaml` to `Languages & Framework | Kubernetes`
31. Setup `Project Structure | Platform Settings | SDKs | Add Python SDK from disk | Virtualenv Environment | Existing environment` on `.venv/bin/python`
32. Add `target`, `.venv`, `.gradle`, `proto` to `Editor | File Types | Ignore files and folders`
33. Change `Help | Change Memory Settings | Max Heap Size` to `4096`

### How to set up a VM for Windows 11

1. Open virt-manager
2. Create a new VM
3. Select "Local install media (ISO image or CD-ROM)"
4. Select the ISO image
5. Memory: 32768, CPUs: 32, Disk: 200GiB
6. Select "Customize configuration before install"
7. CPU host-passthrough
8. Socket: 1, Cores: 16, Threads: 2
9. Begin Installation
10. Select Windows11 Pro
11. Press Shift+F10 to solve "This PC can't run Windows 11."
12. Enter `regedit`
13. Go to `HKEY_LOCAL_MACHINE\SYSTEM\Setup`
14. Create a new key named `LabConfig`
15. Create a new DWORD (32-bit) Value named `BypassTPMCheck` in `LabConfig`
16. Set the value to `1`
17. Create a new DWORD (32-bit) Value named `BypassSecureBootCheck` in `LabConfig`
18. Set the value to `1`
19. Create a new DWORD (32-bit) Value named `BypassRAMCheck` in `LabConfig`
20. Set the value to `1`
21. Back to the previous page
22. Resume the installation
23. Press Shift+F10 to bypass the network connection
24. Type `OOBE\BYPASSNRO`
25. Shutdown the VM

After the installation, tweak the VM settings.

1. Open virt-manager
2. Enable XML Editing
3. Select the VM
4. Rename the computer name to `win11_passthrough`
5. Select "Add Hardware"
6. Add PCI Host Device
7. Add USB Host Device
8. Change Video QXL to None
9. Add `<feature policy="disable" name="hypervisor"/>`, `<feature policy="require" name="topoext"/>` to CPU section

Then, follow the steps below.

1. Install Google Chrome
2. Install nvidia driver
3. Install Docker Desktop
4. Set Google Chrome as the default browser
5. Enable Ctrl+Space for IME
6. Disable UAC
7. Decrease key repeat
8. Tweak Taskbar and Desktop
9. Disable Fast Startup
10. `powercfg /H off` at PowerShell
11. Set Windows Terminal as the default terminal
12. Reboot
13. `wsl --install` at PowerShell
14. `generateResolvConf = false`, `generateHosts = false` to `/etc/wsl.conf`
