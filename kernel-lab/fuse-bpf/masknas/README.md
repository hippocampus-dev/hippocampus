# masknas

<!-- TOC -->
* [masknas](#masknas)
  * [Requirements](#requirements)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

masknas is a fuse-bpf NAS that masks PII in file contents on read while keeping every non-target read on the in-kernel ext4 backing path.

## Requirements

- The fuse-bpf VM from `../experiment.sh`; its kernel needs `CONFIG_DEBUG_INFO_BTF=y`, which needs `pahole` on the build host.
- In-VM toolchain (installed by `run-nas.sh`): clang, llvm, gcc, libbpf-dev, bpftool, pkg-config, libelf-dev, zlib1g-dev, make, curl, ca-certificates, and a Rust toolchain.
- ~8 GB free disk in the VM: `qemu-img resize ../../.build/fuse-bpf/images/rootfs.qcow2 +16G`.

## Usage

```sh
$ sudo ./target/release/masknas --backing-directory /srv/nas-backing --mount-directory /srv/nas --sensitive-suffix .csv --sensitive-suffix .txt
```

Every flag defaults to the value shown above.
`--sensitive-suffix` is injected into the BPF program's read-only `tool_config`, which holds a fixed-size array, so each value must be at most 8 bytes and at most 8 may be given.
The match is bytewise on the name's tail rather than on an extension, so `data` matches `bigdata` and `report.CSV` does not match `.csv`.
Masking rewrites each matched span in place with `*`, so a read returns the same length the backing file has; the patterns cover email addresses, 15- and 16-digit card numbers, and `0`- or `+81`-leading phone numbers.

## Development

```sh
$ ../experiment.sh build && ../experiment.sh rootfs && ../experiment.sh run
$ scp -P 2222 -i ../../.build/fuse-bpf/id_ed25519 -r . debian@localhost:masknas
$ ssh -p 2222 -i ../../.build/fuse-bpf/id_ed25519 debian@localhost
$ cd masknas
$ ./run-nas.sh up
$ ./run-nas.sh down
```

`./run-nas.sh build` recompiles alone once a prior `up` has provisioned the VM.
