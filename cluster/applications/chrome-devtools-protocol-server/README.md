# chrome-devtools-protocol-server

<!-- TOC -->
* [chrome-devtools-protocol-server](#chrome-devtools-protocol-server)
  * [Features](#features)
  * [Development](#development)
    * [E2E Testing](#e2e-testing)
<!-- TOC -->

chrome-devtools-protocol-server is a WebSocket server that proxies Chrome DevTools Protocol connections to headless Chrome instances.

## Features

Chromium binds its remote debugging port to loopback, so both ports republish it on every interface.

| Port | Program | For |
|------|---------|-----|
| 59222 | socat | Callers reaching Chromium by IP address |
| 59223 | chrome-devtools-protocol-server | Callers reaching Chromium by name |

Chromium answers 500 unless `Host` is an IP address or localhost, 403 to a WebSocket handshake carrying an `Origin`, and builds `webSocketDebuggerUrl` out of the `Host` it received.
59223 replaces `Host` with the upstream address, drops `Origin`, and puts the caller's own `Host` back into the URLs `/json/*` answers with.

## Development

```sh
$ make all
```

### E2E Testing

```sh
$ make e2e
```
