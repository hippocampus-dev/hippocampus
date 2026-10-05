---
paths:
  - "**/nginx.conf"
  - "**/nginx.conf.tmpl"
---

* When nginx sits behind an Istio sidecar and proxies to a cluster Service, set `proxy_set_header Host` to the destination's `{service}.{namespace}.svc.cluster.local` and mark it `# HACK for istio-proxy` - Envoy routes on the Host header, so forwarding the client's own Host (`$host`, e.g. `localhost:8080`) matches no registry entry and the sidecar answers 502 with an empty body
* Keep `X-Forwarded-Host` on `$host` so the application still receives the client's Host

## Host Rewrite Placement

| Upstreams in the file | Placement |
|-----------------------|-----------|
| Every proxying `location` targets one Service | `server` block, alongside the other `proxy_set_header` directives |
| Each `location` targets a different Service | Inside each `location`, next to its `proxy_pass` |

Examples: `cluster/manifests/mattermost/overlays/dev/files/nginx.conf.tmpl` (single upstream), `cluster/manifests/mimir/base/files/nginx.conf` and `cluster/manifests/loki/base/files/nginx.conf` (per-location)
