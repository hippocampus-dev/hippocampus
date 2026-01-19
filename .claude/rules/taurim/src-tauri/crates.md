---
paths:
  - "taurim/src-tauri/crates/**/android/**"
---

* Tauri plugins providing Android-native functionality

## Plugin Development

| Location | Purpose |
|----------|---------|
| `src-tauri/crates/tauri-plugin-{name}/` | Rust plugin interface |
| `src-tauri/crates/tauri-plugin-{name}/android/` | Kotlin implementation |

When adding a new plugin:

1. Use `Log.d("{PluginName}", ...)` with consistent tag name in Kotlin code
2. Add the log tag to `Makefile` `android-log` target

## Makefile Targets

| Target | Purpose |
|--------|---------|
| `make android-dev` | Run app on emulator/device |
| `make android-log` | Stream filtered logcat |

## Logging

Log tags in `android-log` must match plugin names:

| Plugin | Log Tag |
|--------|---------|
| `tauri-plugin-gemini` | `GeminiPlugin` |
