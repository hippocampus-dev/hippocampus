# insight

<!-- TOC -->
* [insight](#insight)
  * [Features](#features)
  * [Requirements](#requirements)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

insight is an eBPF-based system monitoring tool that provides full observability for a process.

## Features

- Trace TCP connections
- Observe HTTP/HTTPS requests, responses
- Collect CPU distributions
- Show CPU usage
- Show Memory usage

## Requirements

- bpftool
- clang
- libelf-dev

## Usage

Generate vmlinux.h:

```sh
$ bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h
```

List available tracepoints:

```sh
$ sudo cat /sys/kernel/debug/tracing/available_events
```

Trace function calls containing syscall for kprobe:

```sh
$ sudo perf ftrace --inherit -p <PID>
```

Trace syscall with arguments:

```sh
$ sudo strace -f -p <PID>
```

Inspect an offset of the function for uprobe:

```sh
$ readelf -s -C --wide <PATH>
```

Using bpftrace:

```sh
$ sudo bpftrace -e 'uprobe:/usr/lib/libssl.so.1.1:SSL_read { printf("%r", buf(arg1, arg2)); }'
```

## Development

```sh
$ make dev
```
