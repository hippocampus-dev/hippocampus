# kernel-lab

<!-- TOC -->
* [kernel-lab](#kernel-lab)
  * [Requirements](#requirements)
  * [Usage](#usage)
    * [Adding an experiment](#adding-an-experiment)
<!-- TOC -->

kernel-lab is a reusable harness for building a custom Linux kernel and booting it in a throwaway QEMU VM reachable over SSH.

`lib.sh` holds the shared machinery; each experiment is a subdirectory whose
`experiment.sh` sets the kernel tree, commit, patches, and config, then sources
`lib.sh`. `fuse-bpf/` builds the out-of-tree fuse-bpf series on `bpf-next`;
`sanitizer/` needs no patches and instead builds `v7.1` under KASAN and lockdep,
which cost too much to leave on in a kernel you daily-drive.

## Requirements

- qemu-system-x86_64
- xorriso
- wget
- git
- ssh-keygen
- gcc
- pahole

## Usage

```sh
$ ./fuse-bpf/experiment.sh build    # fetch + patch + build .build/fuse-bpf/linux/arch/x86/boot/bzImage
$ ./fuse-bpf/experiment.sh rootfs   # download rootfs + build the cloud-init SSH seed ISO
$ ./fuse-bpf/experiment.sh run      # boot the VM; Ctrl-C stops it
$ ssh -p 2222 -i .build/fuse-bpf/id_ed25519 debian@localhost
```

Each experiment's artifacts live in `.build/<name>/` (gitignored).

### Adding an experiment

Copy an existing experiment to `<name>/`, add any patches, and edit `<name>/experiment.sh`:

```bash
name="<name>"
tree_url="<git kernel tree>"
base_commit="<pinned commit>"
patches=(...)          # optional mbox files to git am
config=(CONFIG_X=y)    # options appended onto defconfig; each =y is asserted after olddefconfig
```

`build` fails loudly if a requested `CONFIG_*=y` is dropped by an unmet dependency.
