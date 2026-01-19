# Proxy-WASM Filter Patterns

Patterns for developing Envoy proxy-wasm filters.

## Phase Limitations

Proxy-wasm cannot modify request headers during the body phase (`on_http_request_body`). Calling `set_http_request_header` in this phase returns `Status::BadArgument` (status 2).

| Phase | Can Read Headers | Can Modify Headers | Can Use Shared Data |
|-------|------------------|-------------------|---------------------|
| `on_http_request_headers` | Yes | Yes | Yes |
| `on_http_request_body` | Yes | No | Yes |

### Action::Pause Behavior

`Action::Pause` in `on_http_request_headers` maps to Envoy's `StopIteration`, which stops processing and does NOT buffer the body. This means `on_http_request_body` will not be called.

| Return Value | Envoy Behavior | Body Callback |
|--------------|----------------|---------------|
| `Action::Continue` | Continue | Called |
| `Action::Pause` | StopIteration | NOT called |

Note: Lua filters use `StopIterationAndBuffer` which allows body processing after header pause. Proxy-wasm does not have an equivalent action.

## Shared Data Between Filters

Shared data allows filters to pass values between each other. However, filter chain position does not change when phases execute.

### Important Constraint

Header-setter runs in `on_http_request_headers`, which executes **before** any body-phase filters. Therefore, header-setter cannot read values computed from request body. Use ext_proc for body-to-header use cases.

| Use Case | Works? | Reason |
|----------|--------|--------|
| header-getter -> header-setter | Yes | Both run in header phase |
| body-phase filter -> header-setter | No | Body phase runs after header phase |

For body-to-header use cases, use Envoy External Processor (ext_proc). See `.claude/rules/.reference/rust/ext-proc.md`.

### Shared Data Key

Use `context_id` as the shared data key for per-request isolation:

```rust
fn store_value(&self, value: &str) {
    let key = self.context_id.to_string();
    let mut data = if let (Some(bytes), _) = self.get_shared_data(&key) {
        serde_json::from_slice(&bytes).unwrap_or_else(|_| serde_json::json!({}))
    } else {
        serde_json::json!({})
    };
    data["my-key"] = serde_json::Value::String(value.to_string());
    self.set_shared_data(&key, Some(data.to_string().as_bytes()), None).ok();
}
```

### Filter Phase Order

| Phase | Filters | Purpose |
|-------|---------|---------|
| `on_http_request_headers` | header-getter, header-setter | Read/write request headers |
| `on_http_request_body` | (none - use ext_proc for body processing) | Process request body |
| `on_log` | finalizer | Clean up shared data |

Example: `cluster/applications/proxy-wasm/packages/header-setter/`
