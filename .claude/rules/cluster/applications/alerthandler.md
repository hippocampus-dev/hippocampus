---
paths:
  - "cluster/applications/alerthandler/**"
---

* Match an existing GitHub issue on its title over `Issues.ListByRepo` - `Search.Issues` draws on a quota separate from the core limit and reads an asynchronous index that omits issues created seconds earlier, and a `Labels` filter never matches issues whose labels `Issues.Create` silently dropped for lack of push access
* Change the `[SEVERITY] alertname` shape `buildTitle` produces only together with the consumers outside this module that match on it - `.github/workflows/10_triage-loganomaly.yaml` gates its job on `startsWith(github.event.issue.title, '[CRITICAL] loganomaly_')`, so a reworded prefix or separator leaves that workflow never firing again and a job whose `if:` is false is reported nowhere
* Keep `listOpenIssues` filtered to open issues - `.github/workflows/10_triage-loganomaly.yaml` closes the issue it triaged once the failure is over so that the next occurrence opens a fresh one, and a listing widened to closed issues matches that closed issue by title again, leaving `recordOccurrence` to drop every repeat already carrying its `error_hash` label with nothing but its `Skipped commenting` log line to show for it
