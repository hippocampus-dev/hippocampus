# taurim

<!-- TOC -->
* [taurim](#taurim)
  * [Features](#features)
    * [Auto-update](#auto-update)
  * [Requirements](#requirements)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

taurim is a Tauri-based Android utility application.

## Features

- Multi-timer with card stack UI and swipe navigation
- Background foreground service timer with alarm notifications
- Voice control while the alarm rings: saying 止めて, ストップ or stop dismisses it, and on-device Gemini AI (tauri-plugin-gemini) decides whether other phrases ask to stop it
- Timer group save/load
- Wake-on-LAN magic packet to a MAC address on the current LAN, sent on demand or daily at a set time while the phone is on that LAN, the daily alarm surviving reboots and updates, asking for local network access on Android 17+ when Wake Now is tapped or Daily is saved
- Auto-update in release builds from tauri-releases, or from the latest release without signing in in hippocampus-dev/hippocampus builds, described in `### Auto-update`

### Auto-update

| Stage | Behavior |
|-------|----------|
| Check | At startup, retried a minute after each failure up to 60 times while, where tauri-releases serves the update, a sign-in token is saved, stopping when a downloaded APK fails its signature check |
| Sign-in | Only where tauri-releases serves the update, opens the browser for Google sign-in when no token is saved, waiting up to 10 minutes for tauri-releases to return through an App Link that Android verifies against tauri-releases when taurim is installed |
| Rejected token | Forgotten once tauri-releases rejects it, so the next launch signs in again |
| Install | Without confirmation on Android 12+ once no timer is running or paused and no alarm rings, which closes the app |

| Prerequisite | Reason |
|--------------|--------|
| Allowing taurim to install unknown apps | taurim opens that setting once per launch and installs nothing until it is allowed |
| A manually installed release with this feature | Earlier releases do not update themselves |
| Where tauri-releases serves the update, installing that release only after tauri-releases serves `/.well-known/assetlinks.json` | Android 14 and lower verify the App Link only at install |

## Requirements

- Android Studio or Android SDK Command Line Tools
- NDK (Side by side) via SDK Manager
- Java Development Kit (JDK) 25+

## Usage

```sh
$ export ANDROID_HOME=~/Android/Sdk
$ make android-init
$ $ANDROID_HOME/platform-tools/adb pair <ip>:<pairing-port> <pairing-code>
$ $ANDROID_HOME/platform-tools/adb connect <ip>:<port>
$ make android-dev
```

| Step | On the phone |
|------|--------------|
| Show Developer options | Tap Build number under Settings > About phone seven times |
| Turn on Wireless debugging | Developer options > Wireless debugging |
| `adb pair`, once per phone | Wireless debugging > Pair device with pairing code shows `<ip>:<pairing-port>` and `<pairing-code>` |
| `adb connect`, only when `adb devices` lists no phone | The Wireless debugging screen shows `<ip>:<port>`, which changes each time Wireless debugging is turned on |

`make android-dev` installs the debug build as `kaidotio.github.io.debug`, beside the released `kaidotio.github.io` rather than over it.

## Development

```sh
$ make dev
```
