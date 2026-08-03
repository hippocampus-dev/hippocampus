# loganomaly

<!-- TOC -->
* [loganomaly](#loganomaly)
  * [Development](#development)
<!-- TOC -->

loganomaly is a log anomaly detector that consumes Kafka log streams, flagging fatal patterns immediately and error-count spikes by z-score, and emits the results to a Kafka topic.

## Development

```sh
$ make dev
```
