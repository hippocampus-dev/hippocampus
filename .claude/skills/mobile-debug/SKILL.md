---
name: mobile-debug
description: Debugging this repository's mobile apps (taurim) on a real Android phone over Wireless debugging. Use when guiding the user through installing a debug build with make android-dev, pairing or connecting adb, reading device logs with make android-log, or checking a feature such as Wake-on-LAN or timers on the device.
keywords: adb, logcat, android-dev, android-log, 実機, ワイヤレスデバッグ
---

* Leave the released app installed, since the debug build goes in beside it as `## Usage` in `taurim/README.md` states - the release is signed by CI alone, so uninstalling it also drops its data and leaves it for a manual reinstall
* Hand `adb pair` to the user - its pairing code appears only on the phone's screen - while `$HOME/Android/Sdk/platform-tools/adb` itself runs from inside the claudex sandbox
* Run `adb` only after `(exec 3<>/dev/tcp/127.0.0.1/5037)` succeeds - with no server listening, `adb` starts one inside the sandbox on that same port, and the user's `adb pair` against it fails with `protocol fault (couldn't read status message): Success`
* Test anything that stops this machine (waking it through Wake-on-LAN) through a path that needs no WebView, such as the daily alarm - the debug build loads its frontend from this machine's Vite server through `adb reverse`, so the app's screen stops loading once the machine is down

## Workflow

| Step | Who | Action |
|------|-----|--------|
| 1 | User | Connects the phone over Wireless debugging, never USB, running `adb pair` and `adb connect` as `## Usage` in `taurim/README.md` lays out |
| 2 | Agent | Confirms the phone is listed as `device` exactly once by `$HOME/Android/Sdk/platform-tools/adb devices`, dropping an extra `<ip>:<port>` entry with `adb disconnect` when the paired phone has also connected itself as `adb-*._adb-tls-connect._tcp` |
| 3 | User | Runs `make android-dev`, which keeps running until stopped, to install and launch the debug build |
| 3a | Agent | Runs `adb shell pm grant kaidotio.github.io.debug android.permission.ACCESS_LOCAL_NETWORK` once the app is installed, then relaunches it with `adb shell am force-stop` and `adb shell am start -n kaidotio.github.io.debug/kaidotio.github.io.MainActivity` - the CLI points the debug build at this machine's LAN address, which Android 17 blocks for a `targetSdk` 37 app without that permission, leaving a black screen and then `CONNECTION_REFUSED` on `tauri.localhost` with `custom protocol timed out` in logcat |
| 4 | User, Agent | The user operates the feature under test while the agent reads its plugin's tag from `ANDROID_HOME=$HOME/Android/Sdk make android-log` |
| 5 | User | Turns off whatever the test scheduled in the debug build (Wake-on-LAN Daily), since its alarms are separate from the released app's and keep firing after development stops |
