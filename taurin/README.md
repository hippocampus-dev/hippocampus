# taurin

<!-- TOC -->
* [taurin](#taurin)
  * [Features](#features)
  * [Development](#development)
<!-- TOC -->

taurin is a Tauri-based desktop utility application built with Rust backend and Preact frontend.

## Features

- Global shortcut to toggle the application window
- AI-powered monitor explanation (screenshot analysis via OpenAI-compatible API)
- Realtime translation
- GitHub repository creation from a CSV file with a `repository,description,visibility` header, one repository per row that stays private unless `visibility` is `public`, through credential-proxy so no GitHub token is needed, showing the row count and per-row status, running the chosen number of rows at once, and saving a result CSV with `html_url` and `error` appended to every row
- Spotify currently playing track, shown as `track - artists` or `Nothing is playing`, authorized in the browser through hippocampus-dev-spotify-oauth-bridge on first use and again whenever the stored token can no longer be refreshed
- Local voice input using Whisper (speech-to-text with keyboard simulation, desktop only) with floating status indicator
- tmux silence overlay listing every window, marked with the silence alert `monitor-silence` raises, narrowed by a regular expression over `session:window` and the title, and draggable to any position
- Auto-start and auto-update support, checking at startup, retried every minute for up to an hour while it fails, or on demand from the Settings page's `Check for Updates` button, which works even with `Enable Auto Update` off (both require Google sign-in through tauri-releases)
- Persistent settings via tauri-plugin-store

## Development

```sh
$ make dev
```
