---
paths:
  - "**/*.vcl"
---

* Start with `vcl 4.1;`
* When using Varnish with Istio sidecar: rewrite Host header in `vcl_backend_fetch` for correct Envoy routing (Envoy routes based on Host header, but Varnish forwards original request Host by default)

## Header Existence Checks

| Check | `!req.http.X` | `!req.http.X || req.http.X == ""` |
|-------|---------------|-------------------------------------|
| Header missing | true | true |
| Header empty string | false | true |
| Header has value | false | false |

Use `!req.http.X || req.http.X == ""` when empty string headers should be treated as missing.

Note: `std.strlen()` is Fastly-specific and not available in open-source Varnish.

## Istio Integration

| Condition | `vcl_backend_fetch` Pattern |
|-----------|----------------------------|
| Behind ext-proc-proxy (sets `X-Original-Host`) | Restore from `X-Original-Host` with port stripping, fallback to hardcoded service name |
| Direct access (no ext-proc-proxy) | `set bereq.http.Host = "{service}";` |

### Port Stripping

When Istio ServiceEntry maps port 80 to targetPort 443, Envoy appends `:80` to the Host header. Strip it from `X-Original-Host` with `regsub`:

```vcl
set bereq.http.Host = regsub(bereq.http.X-Original-Host, ":80$", "");
```

Apply in both `vcl_recv` (if used for dynamic backend routing) and `vcl_backend_fetch` (when restoring Host). Varnish only receives HTTP plaintext (TLS is terminated by Istio sidecar), so `:443` never appears.

### Cache Invalidation Methods

Use PURGE instead of BAN for cache invalidation when Varnish is behind Envoy/Istio sidecar.

| Method | Works with Envoy | Reason |
|--------|------------------|--------|
| PURGE | Yes | Starts with 'P', recognized by Envoy's HTTP parser |
| BAN | No | Envoy only recognizes methods starting with G, H, P, D, C, O, T |

Example: `cluster/manifests/embedding-gateway/overlays/dev/varnish/files/default.vcl`
