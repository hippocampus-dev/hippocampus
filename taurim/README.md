# taurim

<!-- TOC -->
* [taurim](#taurim)
  * [Features](#features)
  * [Requirements](#requirements)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

taurim is a Tauri-based Android timer application with multi-timer support, background service, alarm notifications, and voice control.

## Features

- Multi-timer with card stack UI and swipe navigation
- Background foreground service timer with alarm notifications
- Voice control while the alarm rings: saying 止めて, ストップ or stop dismisses it, and on-device Gemini AI (tauri-plugin-gemini) decides whether other phrases ask to stop it
- Timer group save/load

## Requirements

- Android Studio or Android SDK Command Line Tools
- NDK (Side by side) via SDK Manager
- Java Development Kit (JDK) 25+

## Usage

```sh
$ export ANDROID_HOME=~/Android/Sdk
$ make android-init
$ make android-dev
```

## Development

```sh
$ make dev
```
