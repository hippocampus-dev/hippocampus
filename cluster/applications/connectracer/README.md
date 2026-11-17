# connectracer

<!-- TOC -->
* [connectracer](#connectracer)
  * [Requirements](#requirements)
  * [Development](#development)
    * [How to generate vmlinux.h](#how-to-generate-vmlinuxh)
<!-- TOC -->

connectracer is an eBPF-based network connection tracer that monitors TCP connections and exports Prometheus metrics.

## Requirements

- bpftool
- clang
- libelf-dev

## Development

### How to generate vmlinux.h

```sh
$ bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h
```

```sh
$ make dev
```
