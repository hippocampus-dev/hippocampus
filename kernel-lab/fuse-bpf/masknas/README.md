# masknas

<!-- TOC -->
* [masknas](#masknas)
  * [Requirements](#requirements)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

masknas is a fuse-bpf NAS that masks PII in file contents on read while keeping every non-target operation on the in-kernel ext4 backing path.

## Requirements

- The fuse-bpf VM from `../experiment.sh`; its kernel needs `CONFIG_DEBUG_INFO_BTF=y`, which needs `pahole` on the build host.
- In-VM toolchain (installed by `run-nas.sh`): clang, llvm, gcc, libbpf-dev, bpftool, pkg-config, libelf-dev, zlib1g-dev, make, curl, ca-certificates, and a Rust toolchain.
- ~8 GB free disk in the VM: `qemu-img resize ../../.build/fuse-bpf/images/rootfs.qcow2 +16G`.

## Usage

The daemon mounts as root and takes the backing path, the mount point, and the suffixes it treats as sensitive from flags.
Every flag defaults to the value shown below, so a bare `sudo ./target/release/masknas` behaves identically; `run-nas.sh` passes the first two explicitly anyway.

```sh
$ sudo ./target/release/masknas --backing-directory /srv/nas-backing --mount-directory /srv/nas --sensitive-suffix .csv --sensitive-suffix .txt
```

`--sensitive-suffix` is injected into the BPF program's read-only `tool_config`, which holds a fixed-size array, so each value must be exactly 4 bytes and at most 8 may be given.
The comparison is bytewise, so `report.CSV` does not match `.csv`.
The name scan stops at 256 bytes, so a longer filename is matched on bytes 253-256 rather than on its real suffix.
Reads of files matching none of the suffixes are answered by the kernel from the ext4 backing path and never reach the daemon.

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
