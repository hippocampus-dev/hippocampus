# masknas

<!-- TOC -->
* [masknas](#masknas)
  * [Requirements](#requirements)
  * [Development](#development)
<!-- TOC -->

masknas is a fuse-bpf NAS that masks PII in file contents on read while keeping every non-target operation on the in-kernel ext4 backing path.

## Requirements

- The fuse-bpf VM from `../experiment.sh`; its kernel needs `CONFIG_DEBUG_INFO_BTF=y`, which needs `pahole` on the build host.
- In-VM toolchain (installed by `run-nas.sh`): clang, llvm, gcc, libbpf-dev, bpftool, pkg-config, libelf-dev, zlib1g-dev, and a Rust toolchain.
- ~8 GB free disk in the VM: `qemu-img resize ../../.build/fuse-bpf/images/rootfs.qcow2 +16G`.

## Development

```sh
$ ../experiment.sh build && ../experiment.sh rootfs && ../experiment.sh run
$ scp -P 2222 -i ../../.build/fuse-bpf/id_ed25519 -r . debian@localhost:masknas
$ ssh -p 2222 -i ../../.build/fuse-bpf/id_ed25519 debian@localhost
$ cd masknas
$ ./run-nas.sh up
$ ./run-nas.sh demo
$ ./run-nas.sh down
```
