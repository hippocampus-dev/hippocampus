---
paths:
  - "cluster/manifests/**/*.alerts.rules"
---

* `message` is the only annotation either receiver renders - omit it and both sides still carry labels and links but no prose at all, so put everything a human must read inside it
* In a `.alerts.rules` file, always write it as a block scalar (`|`), single-line values included - `buildBody` renders it as its own `## Message` section rather than a list item, and every sibling rule in the tree is written this way
* The label set decides how coarse an alert is, because Alertmanager identifies an alert by that set alone, so a label naming a narrower entity splits the alert by that entity - the issue splits with it only for the keys `buildTitle` reads (read `buildTitle` in the same file), and a narrower label outside those keys multiplies alerts that the identical-title guard then collapses into one issue - so choose the set to match the granularity wanted and put anything finer inside `message`
* Attach a label only in the modes where its value varies independently of the labels already present - the slack `text` template renders every pair of `.Labels.SortedPairs` as its own line, so a value that is a pure function of another label (a hash of the grouping the `alertname` already carries) costs a line a reader cannot act on, and the key is left out of the map in those modes rather than set to a placeholder

## Receivers

| Receiver | Renders `message` via | Applies to |
|----------|----------------------|------------|
| slack (Mattermost webhook) | `text` template in `cluster/manifests/mimir/overlays/dev/files/alertmanager.yaml` | every severity |
| alerthandler | `buildBody` in `cluster/applications/alerthandler/handler/critical_alert_handler.go` | `severity: critical` only |

Both `critical` and `warning` route to slack and to alerthandler (`continue: true`), but alerthandler opens a GitHub issue only for `severity: critical` and dispatches the rest on `alertname`.
An alert that needs an automated handler must therefore not carry `severity: critical`.
