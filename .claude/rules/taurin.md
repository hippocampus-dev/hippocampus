---
paths:
  - "taurin/**"
---

* Use `LogicalSize` (not `PhysicalSize`) when calling `window.setSize()` with DOM measurements (`offsetWidth`, `offsetHeight`)
* Set `minWidth` and `minHeight` in `tauri.conf.json` as safety net for dynamic window sizing
* Place desktop-only native dependencies under `[target."cfg(not(any(target_os = \"android\", target_os = \"ios\")))".dependencies]` in `Cargo.toml`
* Use `Feature#Setting` naming convention for store keys (e.g., `"Voice Input#Shortcut"`, `"Realtime Translation#Language"`)

## Window Sizing

| Size Type | Input Unit | Use Case |
|-----------|------------|----------|
| `LogicalSize` | CSS pixels | DOM measurements (`offsetWidth`, `offsetHeight`) |
| `PhysicalSize` | Device pixels | Screen/display APIs |

DOM APIs return CSS pixels (logical), which match `LogicalSize`. Using `PhysicalSize` with DOM measurements causes incorrect sizing on high-DPI displays.

## Platform-Conditional Dependencies

| Dependency Type | Location in Cargo.toml |
|-----------------|------------------------|
| Cross-platform (tauri, serde, tokio) | `[dependencies]` |
| Desktop-only native (cpal, whisper-rs, enigo, global-shortcut) | `[target."cfg(not(any(target_os = \"android\", target_os = \"ios\")))".dependencies]` |

Desktop-only dependencies that use native system APIs (audio capture, keyboard simulation, global shortcuts) fail to compile for Android/iOS targets. Always gate them with the target configuration.

## Settings Store

| Key Format | Example | Purpose |
|------------|---------|---------|
| `Feature#Setting` | `"Voice Input#Model"` | Feature-scoped setting |
| `Setting` | `"Auto Start"` | Top-level setting |

Match keys between `store.get()` in `setup()`, `get_settings()`, and `handle_*()` functions. Read existing keys from `src-tauri/src/commands/settings.rs`.
