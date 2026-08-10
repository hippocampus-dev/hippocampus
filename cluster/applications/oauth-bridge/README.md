# oauth-bridge

<!-- TOC -->
* [oauth-bridge](#oauth-bridge)
  * [Features](#features)
  * [Development](#development)
<!-- TOC -->

oauth-bridge is an HTTP service that proxies OAuth 2.0 authorization code and device code flows for multiple providers, storing encrypted tokens in Redis.

## Features

- [x] Google OAuth 2.0 (authorization code, token refresh)
- [x] Slack OAuth 2.0 (authorization code)
- [x] Spotify OAuth 2.0 (authorization code, token refresh)
- [x] Downstream PKCE (S256) on `/authorize` and `/token`
- [x] Upstream PKCE (S256): a verifier is issued per request and each provider chooses whether to use it

## Development

```sh
$ export CLIENT_ID=<oauth-client-id>
$ export CLIENT_SECRET=<oauth-client-secret>
$ export CALLBACK_URL=<https://oauth-bridge.example.com/callback>
$ export BASE_URL=<https://oauth-bridge.example.com>
$ make dev google  # or slack, spotify
```
