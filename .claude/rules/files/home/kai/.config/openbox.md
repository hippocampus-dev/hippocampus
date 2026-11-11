---
paths:
  - "files/home/kai/.config/openbox/rc.xml"
  - "files/home/kai/.tmux.conf"
  - "taurin/src-tauri/src/main.rs"
  - "taurin/src-tauri/src/commands/settings.rs"
  - "files/etc/systemd/user/teleput-server.service"
  - "files/etc/systemd/user/confidant.service"
  - "moonlander/keymap.c"
---

* Read every file in this rule's `paths` and `files/home/kai/.config/JetBrains/*/keymaps/Mine.xml` before choosing a key for a new shortcut - openbox `<keybind>`s, taurin's global shortcuts, teleput's `--lock-key` and confidant's `--switch-key` and `--confine-key` are each grabbed for the whole X session, so a key one of them already takes never reaches the others or the focused application, and nothing reports the collision
* Pick the modifiers from `## Modifier Patterns` by what the shortcut does
* Choose a key `moonlander/keymap.c` already sends - a shortcut on a key the keymap lacks stays unreachable until the keymap is changed and the firmware flashed
* Bind a dedicated button on another input device (the `XF86Audio*` keys of the ELECOM Relacon) to the key it sends, with no modifier - the device fixes that key, so `## Modifier Patterns` and `moonlander/keymap.c` do not apply, while the collision check above still does

## Modifier Patterns

| Pattern | Purpose | Examples |
|---------|---------|----------|
| `W-S-letter` (openbox) | A desktop action run as a command | `W-S-s` (`screenshot`), `W-S-e` (`toggle-easyeffects`), `W-S-r` (`dunstctl history-pop`) |
| `W-A-key` (openbox) | Spotify playback | `W-A-Return` (`spotify-toggle`), `W-A-n` (`spotify-now-playing`) |
| `W-arrow` (openbox) | Window placement | `W-Up` (Maximize) |
| `A-key` (openbox) | Window switching and the root menu | `A-Tab` (NextWindow), `A-m` (ShowMenu) |
| `C-A-key` (confidant) | Window switching limited to the current monitor | `C-A-Tab` (`--switch-key`) |
| `A-key`, `A-S-key` (taurin) | Defaults of taurin features, which the user can change from its settings | `A-Space`, `A-S-Space` (`"Voice Input#Shortcut"`) |
| `C-q` prefix then a key (tmux) | tmux only, so it collides with nothing outside tmux | `C-q r` (refresh-client) |
| A key with no modifier (teleput) | Pointer lock to the current host, which teleput grabs only with the modifiers given to `--lock-modifier`, so it must be a key no application uses | F13 |
| `C-` with teleput's key (confidant) | Pointer confinement to the current monitor, next to teleput's lock to the current host | `C-F13` (`--confine-key` and `--confine-modifier`) |

## Avoid

| Pattern | Reason |
|---------|--------|
| A new global `A-letter` | `Mine.xml` gives `alt + letter` to JetBrains' most frequent actions, and a global grab takes the key from it |
| `C-A-F1` through `C-A-F12` | `/usr/share/X11/xkb/symbols/srvr_ctrl` maps them to `XF86_Switch_VT_1` through `XF86_Switch_VT_12`, so the X server switches the virtual terminal instead of running the binding |
