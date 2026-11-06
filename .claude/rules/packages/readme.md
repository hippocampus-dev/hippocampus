---
paths:
  - "packages/*/README.md"
---

* Describe the package as it works on any machine, and leave the scripts and units of this repository's own hosts (`files/`, `setup/`) out of its `README.md` - a package is consumed apart from those hosts, by other crates in the workspace or as a release binary (`.github/workflows/20_teleput.yaml`), so a host-side script its README names means nothing to that reader and goes stale whenever the script changes; the host-side walkthrough belongs in `docs/installation.md`, which names the package instead, while a package it depends on stays linkable (`[singleflight](../singleflight)` in `packages/swr/README.md`)
