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
- Voice control with keyword matching and Gemini AI fallback
- Timer group save/load
- On-device AI for intent parsing (tauri-plugin-gemini)

## Requirements

- Android Studio or Android SDK Command Line Tools
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
