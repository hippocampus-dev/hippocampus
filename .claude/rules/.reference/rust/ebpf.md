# eBPF Userspace Patterns

Patterns for processing eBPF events in userspace Rust code.

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
