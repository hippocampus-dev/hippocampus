---
paths:
  - "cluster/manifests/node-local-dns/base/files/Corefile.base"
---

* Never carry `force_tcp` across to the `.:53` block for symmetry with the three cluster-zone blocks - those reach kube-dns while `.:53` reaches whatever the node's own `/etc/resolv.conf` names, upstream's `cluster/addons/dns/nodelocaldns/nodelocaldns.yaml` carries the same asymmetry deliberately, and neither `kustomize build` nor an ArgoCD sync reports the divergence, so it surfaces only as an unrelated workload failing to reach its own dependencies
* Keep `prefer_udp` on that block rather than merely omitting `force_tcp` - `plugin/pkg/proxy/connect.go` falls back to the client's own transport when neither option is set, so a pod asking over TCP still reaches dnsmasq over TCP, and `plugin/forward/forward.go` retries a truncated answer over TCP only where `prefer_udp` is set
