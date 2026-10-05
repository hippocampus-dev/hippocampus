# loganomaly

<!-- TOC -->
* [loganomaly](#loganomaly)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

loganomaly is a log anomaly detector that consumes Kafka log streams, flagging fatal patterns immediately and error-count spikes by z-score, and emits the results to a Kafka topic.

## Usage

Annotate a pod or a namespace to exclude a container's records from both detections.

```yaml
loganomaly.kaidotio.github.io/my-container.exclude: "true"
loganomaly.kaidotio.github.io/my-container.exclude-regex: "Failed to adjust OOM score"
```

| Key | Effect |
|-----|--------|
| `exclude` | Drops every record the named container logs when the value is exactly `true` |
| `exclude-regex` | Drops the records the pattern matches anywhere; the pattern sees the whole JSON document wherever `filter_structural_json` in fluentd-aggregator dropped the `message` field |

Both the pod's annotation and the namespace's are read, and either one alone is enough.
The z-score window is keyed by workload rather than by container, so an exclusion shifts the baseline the rest of that workload is measured against for as many evaluation intervals as the retained samples cover.
Adding one only suppresses over that transient, while removing one fabricates, since the retained samples are then the near-zero counts the exclusion produced and their standard deviation floors at one.
A pattern that fails to compile excludes nothing and reports only to the log.
fluentd delivers an annotation only while `annotation_match` in `cluster/manifests/fluentd/base/files/kubernetes.conf` selects its key.
No annotation excludes a journald record; only `immediateExclusions` in `pkg/consumer/consumer.go` does.
See `examples/`.

## Development

```sh
$ make dev
```
