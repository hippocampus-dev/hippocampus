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
- Local voice input using Whisper (speech-to-text with keyboard simulation, desktop only) with floating status indicator
- Auto-start and auto-update support
- Persistent settings via tauri-plugin-store

## Development

```sh
$ make dev
```
