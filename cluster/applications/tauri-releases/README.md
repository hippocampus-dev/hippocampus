# tauri-releases

<!-- TOC -->
* [tauri-releases](#tauri-releases)
  * [Features](#features)
  * [Development](#development)
<!-- TOC -->

tauri-releases is a release distribution server that serves Tauri application update manifests.

## Features

- [x] Google sign-in (OpenID Connect with PKCE) on `/login`, limited to verified addresses listed in `ALLOWED_EMAILS`
- [x] Session cookie handed back to a loopback `redirect_uri` (`http://127.0.0.1:{port}`) with the caller's `state`
- [x] Session cookie required as `Authorization: Bearer` on every host other than `tauri-releases.kaidotio.dev`

## Development

```sh
$ make all
```
