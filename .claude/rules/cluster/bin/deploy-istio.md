---
paths:
  - "cluster/bin/deploy-istio.sh"
---

* Reach for `proxyStatsMatcher.inclusionSuffixes` rather than `inclusionRegexps` wherever a tail match suffices - Envoy compiles `safe_regex` to a full-string match, so a pattern written for partial matching selects nothing while `istioctl install` and proxy startup both succeed, whereas a suffix is anchored by construction
