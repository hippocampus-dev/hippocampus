# teleput

<!-- TOC -->
* [teleput](#teleput)
  * [Features](#features)
  * [Requirements](#requirements)
  * [Usage](#usage)
<!-- TOC -->

teleput is a keyboard and mouse sharing tool that forwards input from an X11 host to a Wayland host on the LAN.

## Features

- [x] Mutual TLS with certificates pinned at pairing time
- [x] Move between the hosts by crossing the shared screen edge, or toggle with Scroll Lock
- [x] Keyboard, mouse buttons, absolute pointer motion and wheel
- [ ] Clients with more than one screen region
- [ ] Clipboard

## Requirements

| Side | Requirement |
|------|-------------|
| Server | X11 with XInput 2.2, XFixes and XKB |
| Client | A compositor offering `ConnectToEIS` on the RemoteDesktop portal, with an absolute pointer device |

Both sides are assumed to use the same keyboard layout, since keys travel as evdev keycodes.

## Usage

Run the server in the X11 session and the client in the Wayland session with the server's address, where a `.local` name keeps working when the server's IP address changes.

```sh
$ teleput server --edge right
$ teleput client arch.local:24801
```

The first connection pairs the two machines, where each side asks to answer `y` after checking that the fingerprints printed as `peer` and `this machine` match across the two machines, and deleting `$XDG_CONFIG_HOME/teleput/peer.sha256` on the side that rejects a peer whose certificate changed and restarting that side pairs them again on the next connection.

The client asks for the RemoteDesktop permission on its first run and keeps the restore token under `$XDG_STATE_HOME/teleput/`.
