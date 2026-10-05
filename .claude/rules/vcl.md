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

## Cache Status Header

Set `resp.http.X-Cache` in `vcl_deliver`.

| Condition | `X-Cache` |
|-----------|-----------|
| `obj.uncacheable` | `PASS` |
| `obj.hits > 0` | `HIT` |
| Neither | `MISS` |

A two-value `HIT`/`MISS` form files pass, hit-for-pass and hit-for-miss together under `MISS`, leaving a hit rate read from the header divided by a count that holds requests the cache was never allowed to serve.
Add the block to `cluster/manifests/utilities/varnish/files/default.vcl` as well as to every overlay replacing it through `behavior: replace` - an overlay that leaves out `sub vcl_deliver` drops the header with both `kustomize build` and `varnishd -C` still passing, and the base is what the next overlay starts from.
The header is read from the access log, so add `"x-cache": "%RESP(X-CACHE)%"` to the `json_format` in the same overlay's `envoy_filter.yaml` wherever that block goes in - an overlay carrying only the VCL half sets a header nothing collects, with `varnishd -C` and `kustomize build` both passing and a Loki query grouping on the field returning no rows for that workload rather than an error.

## Istio Integration

| Condition | `vcl_backend_fetch` Pattern |
|-----------|----------------------------|
| `X-Original-Host` names the origin Varnish itself fetches from (the ext-proc-proxy Lua sets it from the incoming `Host`) | Restore from `X-Original-Host` with port stripping, fallback to hardcoded service name |
| `X-Original-Host` names a host beyond the backend, which the backend reaches on its own (tls-intercept-proxy sets it, and an ext-proc-proxy in front of Varnish leaves it alone) | `set bereq.http.Host = "{backend service}";` and leave the header for the backend to read |
| No `X-Original-Host` | `set bereq.http.Host = "{service}";` |

Restoring the header into `bereq.http.Host` on the second row sends Envoy a name outside the Sidecar's `egress` list, which a `REGISTRY_ONLY` Sidecar blackholes with nothing failing at build time.

### Port Stripping

When Istio ServiceEntry maps port 80 to targetPort 443, Envoy appends `:80` to the Host header.
Strip it from `X-Original-Host` with `regsub`:

```vcl
set bereq.http.Host = regsub(bereq.http.X-Original-Host, ":80$", "");
```

Apply in both `vcl_recv` (if used for dynamic backend routing) and `vcl_backend_fetch` (when restoring Host).
Varnish only receives HTTP plaintext, so `:443` never appears where the Istio sidecar is what terminates TLS.
Where a TLS-intercepting forward proxy terminates it instead, that proxy decides the form, so read its own normalization rather than assuming a port is there to strip (`cluster/applications/tls-intercept-proxy/main.go`'s `canonicalHost` drops only the default port for the scheme and keeps every other one).

### Cache Invalidation Methods

Use PURGE instead of BAN for cache invalidation when Varnish is behind Envoy/Istio sidecar.

| Method | Works with Envoy | Reason |
|--------|------------------|--------|
| PURGE | Yes | Starts with 'P', recognized by Envoy's HTTP parser |
| BAN | No | Envoy only recognizes methods starting with G, H, P, D, C, O, T |

Example: `cluster/manifests/embedding-gateway/overlays/dev/varnish/files/default.vcl`
