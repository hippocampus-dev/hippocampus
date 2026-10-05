---
paths:
  - "cluster/applications/loganomaly/**"
---

* Change what `buildMessage` in `pkg/adapter/adapter.go` renders into the fenced block only together with the prompt in `.github/workflows/10_triage-loganomaly.yaml`, the only place that describes that block - `severity` sends every immediate detection to the `[CRITICAL] loganomaly_` title the workflow gates its job on, so a description left behind has the triaging agent reading every fenced block as something it no longer is while the adapter, the issue and the triage run all report success
