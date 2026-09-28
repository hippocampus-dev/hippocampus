# varnish-exporter

<!-- TOC -->
* [varnish-exporter](#varnish-exporter)
  * [Requirements](#requirements)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

varnish-exporter is a Prometheus exporter that turns the counters `varnishstat` reports into metrics.

## Requirements

`varnishstat` must be on `PATH` and must come from the same Varnish version as the `varnishd` it reads, since it attaches to a shared memory segment that version wrote.
The `varnishd` working directory (`/var/lib/varnish`) must be readable; a read-only mount is enough.

## Usage

Every scrape runs `varnishstat -1 -j` and converts each counter it reports.
The first field of a counter name becomes part of the metric name, the last field completes it, and the fields between them become the `ident` label.

```
MAIN.n_lru_nuked        -> varnish_main_n_lru_nuked
SMA.s0.g_bytes          -> varnish_sma_g_bytes{ident="s0"}
VBE.boot1.default.fail  -> varnish_vbe_fail{ident="boot1.default"}
```

Counters flagged `c` become Prometheus counters and the rest become gauges, and each counter's description becomes the metric's help text.

```sh
$ varnish-exporter --address=0.0.0.0:9131
```

## Development

```sh
$ make dev
```
