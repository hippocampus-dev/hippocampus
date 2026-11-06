---
paths:
  - "files/home/kai/.config/openbox/rc.xml"
  - "files/home/kai/.tmux.conf"
  - "taurin/src-tauri/src/main.rs"
  - "taurin/src-tauri/src/commands/settings.rs"
  - "files/etc/systemd/user/teleput-server.service"
  - "moonlander/keymap.c"
---

* Read every file in this rule's `paths` and `files/home/kai/.config/JetBrains/*/keymaps/Mine.xml` before choosing a key for a new shortcut - openbox `<keybind>`s, taurin's global shortcuts and teleput's `--lock-key` are each grabbed for the whole X session, so a key one of them already takes never reaches the others or the focused application, and nothing reports the collision
* Pick the modifiers from `## Modifier Patterns` by what the shortcut does
* Choose a key `moonlander/keymap.c` already sends - a shortcut on a key the keymap lacks stays unreachable until the keymap is changed and the firmware flashed

## Modifier Patterns

| Pattern | Purpose | Examples |
|---------|---------|----------|
| `W-S-letter` (openbox) | A desktop action run as a command | `W-S-s` (`screenshot`), `W-S-e` (`toggle-easyeffects`), `W-S-r` (`dunstctl history-pop`) |
| `W-A-key` (openbox) | Spotify playback | `W-A-Return` (`spotify-toggle`), `W-A-n` (`spotify-now-playing`) |
| `W-arrow` (openbox) | Window placement | `W-Up` (Maximize) |
| `A-key` (openbox) | Window switching and the root menu | `A-Tab` (NextWindow), `A-m` (ShowMenu) |
| `A-key`, `A-S-key` (taurin) | Defaults of taurin features, which the user can change from its settings | `A-Space`, `A-S-Space` (`"Voice Input#Shortcut"`) |
| `C-q` prefix then a key (tmux) | tmux only, so it collides with nothing outside tmux | `C-q r` (refresh-client) |
| A key with no modifier (teleput) | teleput grabs it with any modifier, so it must be a key no application uses | F13 |

## Avoid

| Pattern | Reason |
|---------|--------|
| A new global `A-letter` | `Mine.xml` gives `alt + letter` to JetBrains' most frequent actions, and a global grab takes the key from it |
