---
paths:
  - "taurim/src-tauri/crates/**"
---

* Tauri plugins providing Android-native functionality
* Commands must be declared in both `build.rs` COMMANDS array AND `permissions/default.toml`
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

## Command Declaration

Commands must be declared in two places:

| File | Format | Example |
|------|--------|---------|
| `build.rs` | snake_case in COMMANDS | `"start_listening"` |
| `permissions/default.toml` | kebab-case with allow- prefix | `"allow-start-listening"` |

Base Plugin class provides inherited commands that must also be declared:

| Command | Purpose |
|---------|---------|
| `register_listener` | Register event listener channel |
| `remove_listener` | Remove event listener |

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
