# teleput

<!-- TOC -->
* [teleput](#teleput)
  * [Features](#features)
  * [Requirements](#requirements)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

teleput is a keyboard and mouse sharing tool that forwards input from an X11 host to a Wayland, X11 or Windows host on the LAN, shares clipboard text with a Wayland or X11 host and lends the X11 host's microphone to it.

## Features

- [x] Mutual TLS with certificates pinned at pairing time
- [x] Move between the hosts by crossing the shared screen edge, unless the pointer is locked to the current host by pressing the key `--lock-key` names by its X11 keycode, Scroll Lock by default, held with each `--lock-modifier` and no other modifier, until it is pressed again, which leaves the key with other modifiers to other shortcuts and to the client
- [x] One client on each edge given to `--edge`, which can be repeated as `--edge left --edge right`
- [x] Keyboard, mouse buttons, absolute pointer motion and wheel
- [x] Clients with more than one screen region
- [x] Clipboard text in both directions, which the server also relays from one client to the others
- [ ] Clipboard text large enough for X11 to transfer it with INCR
- [x] The server's microphone as a `teleput microphone` source on every client while it is connected

## Requirements

| Side | Requirement |
|------|-------------|
| Server | X11 with XInput 2.2, XFixes and XKB, and `pw-record` for the microphone |
| Client on Wayland | A compositor offering `ConnectToEIS` on the RemoteDesktop portal, with an absolute pointer device, the Clipboard portal or `zwlr_data_control_manager_v1` for the clipboard, and `pw-cat` for the microphone |
| Client on X11 | X11 with XTest and XFixes, and `pw-cat` for the microphone |
| Client on Windows | Running at an integrity level no lower than the windows it controls, since `SendInput` cannot reach a higher one, and neither the clipboard nor the microphone is shared |

Both sides are assumed to use the same keyboard layout, since keys travel as evdev keycodes.

## Usage

Run the server in the X11 session and the client in the Wayland, X11 or Windows session with the server's address, where a `.local` name keeps working when the server's IP address changes and the client takes its session as Wayland when `WAYLAND_DISPLAY` is set.

```sh
$ teleput server --edge right
$ teleput client arch.local:24801
```

The client pins the server's fingerprint in `$XDG_CONFIG_HOME/teleput/peer.sha256` and the server pins the fingerprint of the client on each edge in `$XDG_CONFIG_HOME/teleput/peer.<edge>.sha256`, such as `peer.right.sha256` for `--edge right`.
The first connection pairs the two machines, where the server first asks which edge the client takes while more than one edge is unpaired and each side asks to answer `y` after checking that the fingerprints printed as `peer` and `this machine` match across the two machines.
Deleting the pin file on the side that rejects a peer whose certificate changed and restarting that side pairs them again on the next connection.
Writing the output of `teleput fingerprint` on each machine into the pin file on the other and restarting both pairs them without asking.
The client on Windows keeps these files under `%APPDATA%\teleput\` instead, where `peer.sha256` has to be written as UTF-8 without a BOM.

The client on Wayland asks for the RemoteDesktop permission on its first run and keeps the restore token under `$XDG_STATE_HOME/teleput/`.

## Development

Building the Windows client on Linux takes the `x86_64-w64-mingw32-gcc` cross compiler and the `x86_64-pc-windows-gnu` target.

```sh
$ rustup target add x86_64-pc-windows-gnu
$ cargo build --release -p teleput --target x86_64-pc-windows-gnu
```
