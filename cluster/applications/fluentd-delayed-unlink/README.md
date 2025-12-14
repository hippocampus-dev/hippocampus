# fluentd-delayed-unlink

https://github.com/kubernetes/kubernetes/blob/v1.31.0/pkg/kubelet/kuberuntime/kuberuntime_container.go#L1305

## How to generate vmlinux.h

```sh
$ bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h
```

## Development

```sh
$ make dev
```
