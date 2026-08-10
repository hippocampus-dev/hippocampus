---
paths:
  - "**/*.bpf.c"
---

* Leave a `git check-ignore` hit to the style its upstream chose - this glob reaches the vendored kernel tree under `kernel-lab/.build/` far more often than it reaches a tracked file, and those samples are themselves written in the forms the bullets below rule against
* Treat `insight/src/vfs/vfs.bpf.c`, `insight/src/mysql/mysql.bpf.c` and `insight/src/https/https.bpf.c` as predating these conventions - between them they carry every tracked counter-example below, so reformatting one is a deliberate choice rather than a compliance fix
* Declare maps with the prefix form `SEC(".maps") struct { ... } name;` - upstream libbpf documentation and the kernel samples use the trailing `} name SEC(".maps");` form instead, so a program written by copying either reads as the outlier here
* Order map attributes `type`, then `max_entries` (or `key_size` and `value_size` where the map type takes those), then `key` and `value`, then `map_flags`
* Give a map's capacity a `#define MAX_ENTRIES` rather than a literal at the declaration, keeping a literal only where the number is not a capacity choice at all - a percpu scratch array's `1`, or a ringbuf's `max_entries`, which is a byte size
* Keep `SEC("...")` on the same line as the program signature it annotates
* Spell integer types in program bodies as vmlinux's bare `u8`/`u32`/`u64` alongside `gid_t`/`pid_t`, never `__u8`/`__u32`/`__u64` - the `__`-prefixed spelling belongs only inside `__type(key, ...)` and `__type(value, ...)`, where both spellings are in use
* Open with `#include "vmlinux.h"` ahead of every `<bpf/...>` header, and close the file with `char LICENSE[] SEC("license") = "GPL";` - `insight/src/helpers.h` includes `vmlinux.h` itself, so a program that needs its `log2`/`log2l`/`min` satisfies this through `#include "../helpers.h"`
* Re-`#define` whatever UAPI constant the program needs, in the spelling its uapi header uses - `vmlinux.h` is BTF-generated and carries types only, so `S_IFMT`, `AF_INET`, `PATH_MAX` and the `errno` values are simply absent rather than reachable through some other include
* Take whatever an operator must be able to change at load time from a `const volatile struct { ... } tool_config;` the loader fills, giving it a `= { .field = DEFAULT }` initializer where a compile-time default makes sense - a literal stays inline only where it belongs to the program's definition rather than its configuration, such as a protocol's well-known port or a path prefix the program exists to skip

## Reference

If a variable-length list has to reach the program:
  `cluster/applications/connectracer/src/bpf/connect.bpf.c` pairs a fixed-size array with a `_len` field that the matching loop bounds itself by, while `cluster/applications/fluentd-delayed-unlink/src/bpf/unlink.bpf.c` passes a single NUL-terminated buffer instead. Whether `_len == 0` means "match nothing" or "match everything" is the program's own choice - connectracer's filters return early on it to mean everything.

If writing the loader that fills `tool_config`:
  Read: `.claude/reference/rust/ebpf.md`

If choosing a probe point, or needing `bpf_override_return`:
  Read: `.claude/reference/bpf/probe-points.md`

If writing a fuse-bpf `struct_ops` filter:
  Read: `.claude/reference/bpf/fuse-bpf.md`
