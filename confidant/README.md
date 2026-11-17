# confidant

<!-- TOC -->
* [confidant](#confidant)
  * [Features](#features)
  * [Requirements](#requirements)
  * [Usage](#usage)
    * [Switch](#switch)
    * [Keys](#keys)
    * [Confine](#confine)
  * [Development](#development)
<!-- TOC -->

confidant is a resident desktop utility that switches windows and confines the pointer on its own hotkeys.

## Features

- [x] Switch between the windows on the current monitor on X11
- [x] Switch between the windows on the current monitor on Windows
- [x] Confine the pointer to the monitor showing it on X11 until the key is pressed again
- [x] Confine the pointer to the monitor showing it on Windows until the key is pressed again

## Requirements

| Platform | Requirement |
|----------|-------------|
| X11 | RandR 1.5, XFixes 5, XKB, `fc-match`, and a window manager publishing `_NET_CLIENT_LIST_STACKING` and honoring `_NET_ACTIVE_WINDOW` |
| Windows | Running at an integrity level no lower than the windows it switches to |

## Usage

Run `confidant` in the desktop session, where each key is given as an X11 keycode on X11 and as a virtual-key code on Windows, and any key another client already grabs on X11 fails the start:

```sh
$ confidant --confine-key 191 --confine-modifier control
```

A second `confidant` on Windows exits at once, leaving the running one in place.

### Switch

Ctrl+Alt with `--switch-key`, Tab by default, lists the windows of the current desktop on the monitor of the focused window, or of the pointer when no window has focus.
Windows are listed in stacking order after the focused window, so a window that was focused without being raised keeps its old place, and adding Shift moves backward first.

| Platform | Listed windows |
|----------|----------------|
| X11 | Normal and dialog windows, except those skipping the taskbar, with always-on-top and minimized windows last, each belonging to the monitor showing the largest part of it |
| Windows | Visible unowned windows, with always-on-top windows last |

`--linear` on X11 lists windows in the order they were mapped and starts from the one after the focused window, so repeated short presses rotate through them.
Titles use the bold fontconfig default of `sans` scaled to the resolution of the monitor on X11, and the bold system message font on Windows.

### Keys

| Key | Action |
|-----|--------|
| `--switch-key` | Select the next window |
| Shift with `--switch-key` | Select the previous window |
| Escape | Close the list without switching |
| Releasing Alt | Switch to the selected window |

When Alt is released before the list opens, it switches without showing the list.

### Confine

`--confine-key`, Pause by default, held with each `--confine-modifier` confines the pointer to the monitor showing it until it is pressed again.
On X11, touchscreens and pointer warps still cross the edges of the monitor.

## Development

```sh
$ make dev
```
