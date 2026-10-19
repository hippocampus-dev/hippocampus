---
paths:
  - "cluster/manifests/loganomaly/overlays/dev/cloudevents-alertmanager/service.yaml"
  - "cluster/manifests/loganomaly/overlays/dev/loganomaly-deduplicator/service.yaml"
---

* Keep every suppression window between a detection and the Alertmanager POST below `ALERT_HOLD_DURATION` - `--dedup-ttl` and the consumer's `--suppression-duration` (no manifest sets it, so `DefaultArgs` in `cluster/applications/loganomaly/pkg/consumer/args.go` is what runs) each hold back the push that would extend the alert's `endsAt`, and the widest of them decides, so a window at or above the hold expires the alert while the anomaly continues and the slack receiver posts RESOLVED for something still firing
* Leave headroom above that widest window rather than setting the hold just past it - each window is fixed from the emission that passed and nothing refreshes it, so the next push lands one detection interval after the window expires rather than at the moment it expires
