# tcp-proxy

<!-- TOC -->
* [tcp-proxy](#tcp-proxy)
  * [Features](#features)
  * [Development](#development)
<!-- TOC -->

tcp-proxy is a proxy for TCP workload that supports protocol-aware connection pooling.

## Features

- [x] `raw` hands an upstream connection to a client and never reuses it
- [x] `http` parses HTTP/1.1 and reuses a connection whose exchange completed cleanly
- [x] `redis` parses RESP2 and reuses a connection whose exchange completed cleanly
- [ ] `redis` parses RESP3, so a client has to negotiate RESP2

## Development

```sh
$ make dev
```
