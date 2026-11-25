# tauri-releases

<!-- TOC -->
* [tauri-releases](#tauri-releases)
  * [Features](#features)
    * [Redirect URIs](#redirect-uris)
  * [Development](#development)
<!-- TOC -->

tauri-releases is a release distribution server that serves Tauri application update manifests.

## Features

- [x] Google sign-in (OpenID Connect with PKCE) on `/login`, limited to verified addresses listed in `ALLOWED_EMAILS`
- [x] Session cookie handed back to the caller's `redirect_uri` with its `state`, only after the user presses the button on a page `/login` shows, whose form must be posted from the same origin, as `### Redirect URIs` lists
- [x] `/.well-known/assetlinks.json` served without sign-in so Android can verify taurim's App Link
- [x] Session cookie required as `Authorization: Bearer` on every request outside `/login`, `/callback`, `/taurim/callback`, `/.well-known/assetlinks.json` and `/healthz`

### Redirect URIs

| `redirect_uri` | Caller | Accepted from |
|----------------|--------|---------------|
| `http://127.0.0.1:{port}` | taurin | Browsers whose User-Agent does not name Android |
| `https://{host}/taurim/callback` | taurim, through its App Link | Any browser |

## Development

```sh
$ make all
```
