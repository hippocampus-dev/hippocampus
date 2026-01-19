---
paths:
  - "**/*.vcl"
---

* Start with `vcl 4.1;`
* When using Varnish with Istio sidecar: rewrite Host header in `vcl_backend_fetch` to match backend service name (Envoy routes based on Host header, but Varnish forwards original request Host by default)

## Header Existence Checks

| Check | `!req.http.X` | `!req.http.X || req.http.X == ""` |
|-------|---------------|-------------------------------------|
| Header missing | true | true |
| Header empty string | false | true |
| Header has value | false | false |

Use `!req.http.X || req.http.X == ""` when empty string headers should be treated as missing.

Note: `std.strlen()` is Fastly-specific and not available in open-source Varnish.

## Istio Integration

| Subroutine | Required Action |
|------------|-----------------|
| `vcl_backend_fetch` | `set bereq.http.Host = "{service}";` |

Example: `cluster/manifests/embedding-gateway/overlays/dev/varnish/files/default.vcl`
