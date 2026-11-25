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
- Realtime translation of the desktop audio with gpt-realtime-translate
- GitHub repository creation from a CSV file with a `repository,description,visibility` header, one repository per row that stays private unless `visibility` is `public`, through credential-proxy so no GitHub token is needed, showing the row count and per-row status, running the chosen number of rows at once, and saving a result CSV with `html_url` and `error` appended to every row
- Spotify currently playing overlay, showing album artwork (with a pause overlay while paused), track name, artists, album name, and playback progress, hidden while nothing is playing, showing the error and retried every minute up to 60 times while fetching fails and a Spotify token is saved and then stopped until `Show Currently Playing Overlay` is turned on again, and draggable to any position, authorized in the browser through hippocampus-dev-spotify-oauth-bridge on first use and again whenever the stored token can no longer be refreshed
- Voice input transcribed locally with Whisper (speech-to-text with keyboard simulation, desktop only) with floating status indicator, which shows the result next to the mouse cursor and types it into the focused window only when `y` is pressed and discards it on `n`, while `y`, `n` and with `Enable Rewrite` on `r` reach no application and the voice input shortcut does nothing until it is typed or discarded, biased toward the words and spellings entered in the Settings page's Voice Input `Prompt`, in the CUDA build only, which runs it on the GPU, which the asdf plugin installs on Linux x86_64 when `ldd` resolves all of its libraries (the AppImage without voice input otherwise) and `setup/windows/setup.ps1` and `setup-hyperv-host.sh` (with `setup/hyperv-host/setup.ps1`) install together with the CUDA 13.5 runtime on a Windows with an NVIDIA GPU, and which auto-updates to the CUDA build, where `Enable Rewrite` with the model from `Download Rewrite Model` has Qwen3-4B-Instruct-2507 drop filler words and repetitions, keep only the corrected wording and fix punctuation on an NVIDIA driver that supports the CUDA toolkit the build was compiled with (the transcript as is when the model is still loading or the rewrite fails or passes 5 seconds), and where with `Enable Rewrite` on an instruction spoken while holding `r` replaces the result with the model's revision, without line breaks or tabs, keeping it as is when it is more than 200 tokens, the model is still loading or the revision fails or passes 5 seconds
- tmux silence overlay listing every window, marked with the silence alert `monitor-silence` raises, narrowed by a regular expression over `session:window` and the title, and draggable to any position
- Window position reset from the Settings page's `Reset Window Positions` button, which centers the application window and returns the tmux silence overlay to the top-right corner and the Spotify overlay to the top-left corner
- Auto-start and auto-update support, both registering the login entry and checking at startup in release builds only, retried every minute for up to an hour while it fails and, where tauri-releases serves the update, a sign-in token is saved, forgetting the token once tauri-releases rejects it so the next check signs in again, or on demand from the Settings page's `Check for Updates` button, which works even with `Enable Auto Update` off (both require Google sign-in through tauri-releases, except in hippocampus-dev/hippocampus builds, which fetch that repository's latest release without signing in)
- Chrome Native Messaging host (`dev.kaidotio.taurin`), entered when launched with a `chrome-extension://` argument and registered automatically in release builds for `chrome-extension://phegfihdgknjjilfnlhljhkbljdbpogk/` (and also under `/var/google-chrome/NativeMessagingHosts` on Linux when `/var/google-chrome` exists), dispatching messages by their `type` field (`create_github_repository` and `connect`, both responding with `{}`, forwarding `id`-bearing responses without a `type` over the bridge, and exiting on unknown types or messages without `type` or `id`) and bridging persistent `connect` sessions over a local loopback endpoint (kept apart from the installed application's in debug builds) so the running application can send correlated requests to the extension and receive responses, including the `Ping Extension` launcher item that sends a `ping` request with a 5-second timeout and shows the extension's JSON response (`{"pong": true}`) or error in a dialog
- Persistent settings via tauri-plugin-store, kept apart from the installed application's in debug builds so their shortcuts can differ

## Development

```sh
$ make dev
```

`npm run dev:cuda` runs the desktop application with voice input, which only the CUDA build has, and needs a CUDA toolkit with `nvcc` on `PATH`:

```sh
$ npm run dev:cuda
```
