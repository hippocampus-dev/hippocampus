# promphet

<!-- TOC -->
* [promphet](#promphet)
  * [Development](#development)
<!-- TOC -->

promphet is a CronJob that fetches time-series data from Prometheus, runs StatsForecast (AutoARIMA, AutoETS, AutoTheta median ensemble) forecasting, and pushes predictions to Pushgateway.

## Development

```sh
$ export PROMETHEUS_HOST=<value>
$ export PUSHGATEWAY_HOST=<value>
$ export CONFIG_FILE=<path-to-local-config.yaml>
$ make dev
```
