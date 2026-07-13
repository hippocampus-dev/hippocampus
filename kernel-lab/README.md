# kernel-lab

<!-- TOC -->
* [kernel-lab](#kernel-lab)
  * [Requirements](#requirements)
  * [Usage](#usage)
    * [Adding an experiment](#adding-an-experiment)
<!-- TOC -->

kernel-lab is a reusable harness for building a patched Linux kernel and booting it
in a throwaway QEMU VM reachable over SSH.

`lib.sh` holds the shared machinery; each experiment is a subdirectory whose
`experiment.sh` sets the kernel tree, commit, patches, and config, then sources
`lib.sh`. The first one, `fuse-bpf/`, builds the out-of-tree fuse-bpf series on
`bpf-next`.

## Requirements

- `qemu-system-x86_64`, `xorriso`, `wget`, `git`, `ssh-keygen`, `gcc` (>= 15 ok)
- Access to `git.kernel.org` and `cloud.debian.org`; ~5 GB free disk per experiment

## Usage

```sh
$ ./fuse-bpf/experiment.sh build    # fetch + patch + build .build/fuse-bpf/linux/arch/x86/boot/bzImage
$ ./fuse-bpf/experiment.sh rootfs   # download rootfs + build the cloud-init SSH seed ISO
$ ./fuse-bpf/experiment.sh run      # boot the VM; Ctrl-C stops it
$ ssh -p 2222 -i .build/fuse-bpf/id_ed25519 debian@localhost
```

Each experiment's artifacts live in `.build/<name>/` (gitignored).

### Adding an experiment

Copy `fuse-bpf/` to `<name>/`, drop in its patches, and edit `<name>/experiment.sh`:

```sh
name="<name>"
tree_url="<git kernel tree>"
base_commit="<pinned commit>"
patches=(...)          # optional mbox files to git am
config=(CONFIG_X=y)    # options appended onto defconfig; each =y is asserted after olddefconfig
```

`build` fails loudly if a requested `CONFIG_*=y` is dropped by an unmet dependency.
