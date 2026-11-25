# redis-proxy

<!-- TOC -->
* [redis-proxy](#redis-proxy)
  * [Features](#features)
  * [Development](#development)
<!-- TOC -->

redis-proxy is a proxy for redis that supports read/write splitting and connection pooling.

## Features

- [x] Routes read commands to `--reader-remote-address` when `--reader-routing` is set
- [x] Mirrors write commands to `--mirror-remote-address` without making the client wait, carrying each client connection's `SELECT`, `AUTH`, `HELLO` and transactions over to it
- [x] Reports the error replies of the mirror upstream in `redis_mirror_errors_total` and the commands waiting for it in `redis_mirror_queued_commands`
- [x] Parses RESP2 and reuses a connection whose exchange completed cleanly
- [ ] Parses RESP3, so a client has to negotiate RESP2

## Development

```sh
$ make dev
```
