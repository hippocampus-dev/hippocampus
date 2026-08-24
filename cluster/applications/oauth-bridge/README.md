# oauth-bridge

<!-- TOC -->
* [oauth-bridge](#oauth-bridge)
  * [Features](#features)
  * [Requirements](#requirements)
  * [Development](#development)
<!-- TOC -->

oauth-bridge is an HTTP service that proxies OAuth 2.0 authorization code and device code flows for multiple providers, storing encrypted tokens in Redis.

## Features

- [x] Google OAuth 2.0 (authorization code, token refresh)
- [x] oauth2-proxy (session cookie as the access token)
- [x] Slack OAuth 2.0 (authorization code)
- [x] Spotify OAuth 2.0 (authorization code, token refresh)
- [x] Downstream PKCE (S256) on `/authorize` and `/token`
- [x] Redirect URI restricted to HTTPS or loopback HTTP on `/authorize`
- [x] Upstream PKCE (S256): a verifier is issued per request and each provider chooses whether to use it

## Requirements

- oauth2-proxy provider: `PROVIDER_URL` must point at the upstream oauth2-proxy
- oauth2-proxy provider: `CALLBACK_URL` must be a host that the upstream oauth2-proxy covers with both `--cookie-domain`, so that the session cookie reaches `/callback`, and `--whitelist-domain`, so that `rd` is accepted

## Development

```sh
$ export CLIENT_ID=<oauth-client-id>
$ export CLIENT_SECRET=<oauth-client-secret>
$ export CALLBACK_URL=<https://oauth-bridge.example.com/callback>
$ export BASE_URL=<https://oauth-bridge.example.com>
$ make dev
```
