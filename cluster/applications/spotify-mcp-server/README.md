# spotify-mcp-server

<!-- TOC -->
* [spotify-mcp-server](#spotify-mcp-server)
  * [Features](#features)
  * [Development](#development)
<!-- TOC -->

spotify-mcp-server is a Model Context Protocol server that provides Spotify track API access to AI agents.

## Features

- [x] OAuth 2.0 Protected Resource Metadata (RFC9728) on `/.well-known/oauth-protected-resource`
- [x] `401` with `WWW-Authenticate` on `/mcp` when the bearer token is missing or Spotify rejects it
- [x] `isError` tool results when the Spotify Web API answers with an error status

## Development

```sh
$ make dev
```
