# fluentd-delayed-unlink

<!-- TOC -->
* [fluentd-delayed-unlink](#fluentd-delayed-unlink)
  * [Requirements](#requirements)
  * [Development](#development)
    * [How to generate vmlinux.h](#how-to-generate-vmlinuxh)
<!-- TOC -->

fluentd-delayed-unlink is an eBPF-based tool that prevents race conditions between Kubernetes log rotation and Fluentd processing.

https://github.com/kubernetes/kubernetes/blob/v1.31.0/pkg/kubelet/kuberuntime/kuberuntime_container.go#L1305

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
