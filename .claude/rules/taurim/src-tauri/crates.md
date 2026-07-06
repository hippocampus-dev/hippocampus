---
paths:
  - "taurim/src-tauri/crates/**"
---

* Tauri plugins providing Android-native functionality
* Commands must be declared in three places: `build.rs` COMMANDS array, `permissions/default.toml`, AND `src/lib.rs` `invoke_handler`
* Plugin capabilities must be granted in `capabilities/android.json` for Android platform
* Log user input by length, not content (privacy): `Log.d("Plugin", "received length=${input.length}")`

## Plugin Development

| Location | Purpose |
|----------|---------|
| `src-tauri/crates/tauri-plugin-{name}/` | Rust plugin interface |
| `src-tauri/crates/tauri-plugin-{name}/android/` | Kotlin implementation |
| `src-tauri/crates/tauri-plugin-{name}/build.rs` | Command declaration |
| `src-tauri/crates/tauri-plugin-{name}/permissions/default.toml` | Permission declaration |

When adding a new plugin:

1. Use `Log.d("{PluginName}", ...)` with consistent tag name in Kotlin code
2. Add the log tag to `Makefile` `android-log` target
3. Declare all commands in `build.rs` COMMANDS array
4. Add corresponding permissions in `permissions/default.toml`
5. Add all commands to `src/lib.rs` `invoke_handler(tauri::generate_handler![...])`
6. Grant plugin capability in `capabilities/android.json` with entry `"{plugin-name}:default"`

## Command Declaration

Commands must be declared in four places for Android:

| File | Format | Example |
|------|--------|---------|
| `build.rs` | snake_case in COMMANDS | `"start_listening"` |
| `permissions/default.toml` | kebab-case with allow- prefix | `"allow-start-listening"` |
| `src/lib.rs` | snake_case in `tauri::generate_handler![]` | `start_listening` |
| `capabilities/android.json` | kebab-case with plugin: prefix | `"plugin:default"` or `"speech:default"` |

The capability identifier format is `{plugin-name}:default` where `{plugin-name}` matches the `name` field in the plugin's Cargo.toml.

Base Plugin class provides inherited commands that must also be declared:

| Command | Purpose |
|---------|---------|
| `register_listener` | Register event listener channel |
| `remove_listener` | Remove event listener |

## Native-to-JS Event Pipeline

For real-time events from Android native to JS (instead of polling via commands), use the `trigger()`/`Channel`/`registerListener` pattern:

| Layer | Pattern | Example |
|-------|---------|---------|
| Kotlin | `trigger(eventName, JSObject())` in `BroadcastReceiver.onReceive()` | `trigger("tick", payload)` |
| TypeScript | `createChannel()` + `registerListener()` in `setup{Feature}Listeners()` | `setupTimerListeners()`, `setupVoiceListeners()` |

Copy from: `src/services/voiceService.ts` (Channel/TauriInternals/createChannel/registerListener)

## Makefile Targets

| Target | Purpose |
|--------|---------|
| `make android-dev` | Run app on emulator/device |
| `make android-log` | Stream filtered logcat |

## Logging

Log tags in `android-log` must match plugin names. Check existing plugins:
`ls taurim/src-tauri/crates/tauri-plugin-*/`

| Naming Convention | Example |
|-------------------|---------|
| Plugin: `tauri-plugin-{name}` | Log Tag: `{Name}Plugin` |
