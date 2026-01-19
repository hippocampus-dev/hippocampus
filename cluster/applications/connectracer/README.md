# connectracer

## How to generate vmlinux.h

```sh
$ bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h
```

## Development

```sh
$ make dev
```
