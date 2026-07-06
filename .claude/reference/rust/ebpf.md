# eBPF Patterns

Patterns for eBPF programs and userspace event processing.

## Probe Point Selection

| Use Case | Preferred Pattern | Reason |
|----------|-------------------|--------|
| File I/O (read/write) | `fexit/vfs_read`, `fexit/vfs_write` | Architecture-agnostic, handles all code paths |
| File deletion (observation only) | `kprobe/vfs_unlink` | Architecture-agnostic, only fires for files |
| File deletion (with override) | `kprobe/__x64_sys_unlink*` | Required for `bpf_override_return` |
| Socket operations | `tracepoint/syscalls/sys_enter_*` + arch-specific kprobe | Need userspace pointers from syscall args |

### VFS Hooks vs Syscall Hooks

Prefer VFS-level hooks over syscall-level hooks when possible:

| Hook Level | Pros | Cons |
|------------|------|------|
| VFS (`vfs_*`) | Architecture-agnostic, single hook point | Cannot access userspace pointers directly, not in ALLOW_ERROR_INJECTION |
| Syscall (`__x64_sys_*`) | Access to userspace args, in ALLOW_ERROR_INJECTION | Architecture-specific wrappers needed |
| Tracepoint | Stable ABI, userspace args available | Limited to syscall entry/exit points, cannot override |

### bpf_override_return Limitation

`bpf_override_return` can only be used on functions in the kernel's `ALLOW_ERROR_INJECTION` whitelist. VFS functions (`vfs_unlink`, `vfs_read`, etc.) are NOT in this whitelist.

| Function Type | In ALLOW_ERROR_INJECTION | bpf_override_return |
|---------------|--------------------------|---------------------|
| `vfs_*` | No | Cannot use |
| `__x64_sys_*`, `__arm64_sys_*` | Yes | Can use |
| `do_sys_*` | Some | Check whitelist |

When you need to override syscall return values (e.g., block file deletion), use architecture-specific syscall wrappers:

```c
#if defined(__TARGET_ARCH_x86)
#define SYS_PREFIX "__x64_"
#elif defined(__TARGET_ARCH_s390)
#define SYS_PREFIX "__s390x_"
#elif defined(__TARGET_ARCH_arm64)
#define SYS_PREFIX "__arm64_"
#else
#define SYS_PREFIX "__se_"
#endif

SEC("kprobe/" SYS_PREFIX "sys_unlink") int BPF_KPROBE(sys_unlink) {
    // ... filtering logic ...
    bpf_override_return(ctx, 0);  // Works because __x64_sys_unlink is in whitelist
    return 0;
}
```

### Combined Pattern (Tracepoint + Syscall Kprobe)

When you need both userspace pointers and `bpf_override_return`, use tracepoints to capture arguments and architecture-specific syscall kprobes for the override:

```c
// Tracepoint: capture userspace pathname pointer
SEC("tracepoint/syscalls/sys_enter_unlink") int sys_enter_unlink(...) {
    const char *pathname = (const char *)ctx->args[0];
    struct arg arg = { .pathname = pathname };
    bpf_map_update_elem(&args, &pid, &arg, BPF_ANY);
    return 0;
}

// Syscall kprobe: override return value (architecture-specific)
SEC("kprobe/" SYS_PREFIX "sys_unlink") int BPF_KPROBE(sys_unlink) {
    struct arg *argp = bpf_map_lookup_elem(&args, &pid);
    if (!argp) return 0;
    // Use argp->pathname captured from tracepoint
    bpf_probe_read_user(eventp->pathname, sizeof(eventp->pathname), argp->pathname);
    // Override return value to block the syscall
    bpf_override_return(ctx, 0);
    // ...
}
```

Example: `cluster/applications/fluentd-delayed-unlink/src/bpf/unlink.bpf.c`

### VFS Pattern (Observation Only)

When you only need to observe operations without overriding, prefer VFS-level hooks for architecture independence:

```c
// VFS hook: intercept at VFS level (architecture-agnostic, observation only)
SEC("kprobe/vfs_unlink") int BPF_KPROBE(vfs_unlink) {
    // Can observe but CANNOT use bpf_override_return here
    // ...
}
```

## Event Buffer Handling

| eBPF Function | Buffer Type | Userspace Pattern |
|---------------|-------------|-------------------|
| `bpf_probe_read_user_str` | NUL-terminated | `std::ffi::CStr::from_bytes_until_nul` |
| `bpf_probe_read_user` with len field | Length-prefixed | Slice with min: `&buf[..std::cmp::min(len, buf.len())]` |

### NUL-Terminated Strings

For buffers populated by `bpf_probe_read_user_str`:

```rust
let payload = std::ffi::CStr::from_bytes_until_nul(&event.buf)
    .ok()
    .and_then(|c| c.to_str().ok())
    .map(|s| s.trim_end())
    .unwrap_or("");
```

Example: `insight/src/http.rs`

### Length-Prefixed Buffers

For buffers with explicit length field:

```rust
let payload = if event.len > 0 {
    let end_idx = std::cmp::min(event.len as usize, event.buf.len());
    &event.buf[..end_idx]
} else {
    &[]
};
```

Example: `insight/src/mysql.rs`

### Helper Function Pattern

For fixed-size arrays (comm, filename):

```rust
fn bytes_to_string(bytes: &[u8]) -> String {
    let nul_pos = bytes.iter().position(|&b| b == 0).unwrap_or(bytes.len());
    String::from_utf8_lossy(&bytes[..nul_pos]).into_owned()
}
```

Example: `insight/src/vfs.rs`

### Anti-Pattern

Do NOT use:

```rust
// Wrong: trim_end_matches does not handle embedded NULs correctly
std::str::from_utf8(&event.buf)
    .map(|s| s.trim_end_matches(char::from(0)))
```

This fails when buffer contains garbage after NUL terminator.
