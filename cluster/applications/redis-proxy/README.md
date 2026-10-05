# redis-proxy

<!-- TOC -->
* [redis-proxy](#redis-proxy)
  * [Features](#features)
  * [Development](#development)
<!-- TOC -->

redis-proxy is a proxy for redis that supports read/write splitting and connection pooling.

## Features

- [x] Routes read commands to `--reader-remote-address` when `--reader-routing` is set
- [x] Parses RESP2 and reuses a connection whose exchange completed cleanly
- [ ] Parses RESP3, so a client has to negotiate RESP2

## Development

```sh
$ make dev
```
