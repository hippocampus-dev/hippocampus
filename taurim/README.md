# Taurim - Tauri Timer App

A Tauri-based Android timer application with multi-timer support, background service, alarm notifications, and voice control.

## Prerequisites

### 1. System Requirements
- Node.js 18+ and npm
- Rust 1.70+
- Android Studio or Android SDK Command Line Tools
- Java Development Kit (JDK) 25+

### 2. Android SDK Setup

#### Option A: Install Android Studio (Recommended)
1. Download and install [Android Studio](https://developer.android.com/studio)
2. Open Android Studio and go to **Settings → SDK Manager**
3. Install the following:
   - Android SDK Platform 33 or higher
   - Android SDK Build-Tools
   - Android SDK Platform-Tools
   - Android SDK Command-line Tools
   - Android Emulator (for testing)

#### Option B: Command Line Tools Only
```bash
# Download command line tools from https://developer.android.com/studio#command-tools
mkdir -p ~/Android/Sdk
cd ~/Android/Sdk
unzip commandlinetools-linux-*.zip

# Set environment variables
export ANDROID_HOME=~/Android/Sdk
export PATH=$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools

# Install required packages
sdkmanager "platform-tools" "platforms;android-33" "build-tools;33.0.0"
```

### 3. Environment Variables
Add to your shell configuration (~/.bashrc, ~/.zshrc, etc.):
```bash
export ANDROID_HOME=~/Android/Sdk
export PATH=$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools
export PATH=$PATH:$ANDROID_HOME/emulator
```

## Project Setup

### 1. Install Dependencies
```bash
cd /opt/hippocampus/taurim
npm install
```

### 2. Initialize Android Project
```bash
# After setting up Android SDK
npm run tauri android init
```

This will create the Android project structure in `src-tauri/gen/android/`.

## Building the App

### Development Build
```bash
# Start development server
npm run dev

# In another terminal, run Android dev build
npm run tauri android dev
```

### Production Build
```bash
# Build optimized APK
npm run tauri android build

# The APK will be located at:
# src-tauri/gen/android/app/build/outputs/apk/universal/release/app-universal-release-unsigned.apk
```

### Signed Production Build

For signed releases (required for Play Store or direct distribution), set environment variables before building:

```bash
export ANDROID_KEYSTORE_FILE=/path/to/release.keystore
export ANDROID_KEYSTORE_PASSWORD=your_keystore_password
export ANDROID_KEY_ALIAS=your_key_alias
export ANDROID_KEY_PASSWORD=your_key_password

npm run tauri android build
```

See [APK Signing](#apk-signing) section for keystore creation.

## Installation on Android Device

### 1. Enable Developer Mode on Android Device
1. Go to **Settings → About Phone**
2. Tap **Build Number** 7 times
3. Go back to **Settings → System → Developer Options**
4. Enable **USB Debugging**

### 2. Install via USB
```bash
# Connect your device via USB
adb devices  # Verify device is connected

# Install the APK
adb install -r src-tauri/gen/android/app/build/outputs/apk/universal/release/app-universal-release-unsigned.apk
```

### 3. Install via File Transfer
1. Build the APK as shown above
2. Transfer the APK file to your device
3. Open the file on your device and install (may need to enable "Install from Unknown Sources")

## Debugging

### Using Chrome DevTools (for WebView)
1. Connect device via USB with debugging enabled
2. Open Chrome and navigate to `chrome://inspect`
3. Your app should appear under "Remote Target"
4. Click "inspect" to debug the WebView

### Using ADB Logcat
```bash
# View all logs
adb logcat

# Filter by your app
adb logcat | grep -i taurim

# Clear logs
adb logcat -c
```

### Using Android Studio
1. Open Android Studio
2. Click **File → Open** and select `/opt/hippocampus/taurim/src-tauri/gen/android`
3. Use the built-in debugging tools

## Testing on Emulator

### Create and Start Emulator
```bash
# List available system images
sdkmanager --list | grep system-images

# Install a system image (example: Android 33)
sdkmanager "system-images;android-33;google_apis;x86_64"

# Create AVD (Android Virtual Device)
avdmanager create avd -n TaurimEmulator -k "system-images;android-33;google_apis;x86_64"

# Start emulator
emulator -avd TaurimEmulator
```

### Run App on Emulator
```bash
# With emulator running
npm run tauri android dev

# Or install APK directly
adb install -r path/to/app.apk
```

## Voice Control (Android only)

Taurim supports hands-free voice control on Android devices. Voice recognition starts automatically when the app launches and continuously listens for commands.

### Voice Commands

| Voice Input | Action |
|-------------|--------|
| "start", "go", "begin", "hajime", "suta-to" | Start timer |
| "stop", "pause", "wait", "tome", "sutoppu", "po-zu" | Pause timer |
| "reset", "restart", "clear", "risetto" | Reset timer |
| "next", "skip", "tsugi" | Next timer |
| "use [group name]", "load [group name]" | Load timer group |

Japanese keywords are also supported (in romaji above for readability).

### Voice Indicator

The microphone icon in the top-right corner shows the current voice recognition status:

| Color | Status |
|-------|--------|
| Green (pulsing) | Listening for commands |
| Yellow | Processing speech |
| Red | Error occurred |
| Gray | Idle / Not listening |

### Hybrid Intent Parsing

Voice commands are processed in two stages:
1. **Keyword matching** - Instant response for simple commands
2. **Gemini AI fallback** - Natural language understanding for complex phrases (requires 10+ characters)

This hybrid approach provides immediate feedback for common commands while supporting conversational input like "please start the timer" or "load my morning workout".

## Project Structure

```
taurim/
├── src/                    # Frontend source code
│   ├── main.ts            # Application entry point
│   ├── components/        # Preact UI components
│   │   ├── App.ts         # Main app component
│   │   ├── TimerView.ts   # Timer orchestration and lifecycle
│   │   ├── TimerCardStack.ts  # Multi-timer card stack with swipe navigation
│   │   ├── TimerCard.ts   # Single timer display with progress ring
│   │   ├── TimerInput.ts  # Time adjustment buttons (±10m, ±1m, ±10s, ±1s)
│   │   ├── TimerControls.ts   # Start/Pause/Reset/Stop alarm buttons
│   │   ├── ProgressRing.ts    # Circular progress indicator
│   │   ├── SavedGroups.ts # Timer group management UI
│   │   └── VoiceIndicator.ts  # Microphone status indicator
│   ├── services/          # Application services
│   │   ├── intentService.ts   # Voice command intent parsing
│   │   ├── timerService.ts    # Timer and alarm (Web Audio API)
│   │   └── voiceService.ts    # Speech recognition event handling
│   └── state/             # Application state management
│       ├── timerState.ts  # Timer state signals
│       └── voiceState.ts  # Voice recognition state signals
├── src-tauri/             # Rust backend
│   ├── crates/            # Tauri plugins
│   │   ├── tauri-plugin-gemini/  # On-device AI (categorization + intent parsing)
│   │   ├── tauri-plugin-speech/  # Android speech recognition
│   │   └── tauri-plugin-timer/   # Foreground service timer
│   ├── src/
│   │   ├── main.rs        # Desktop entry point
│   │   └── lib.rs         # Mobile entry point
│   ├── Cargo.toml         # Rust dependencies
│   └── tauri.conf.json    # Tauri configuration
├── index.html             # HTML entry point
├── package.json           # Node dependencies
└── vite.config.js         # Vite configuration
```

## Troubleshooting

### Android SDK Not Found
- Ensure ANDROID_HOME is set correctly
- Verify SDK installation path
- Restart terminal after setting environment variables

### Build Failures
- Clear build cache: `cd src-tauri && cargo clean`
- Update dependencies: `npm update && cd src-tauri && cargo update`
- Check minimum SDK version in tauri.conf.json

### Device Not Recognized
- Ensure USB debugging is enabled
- Try different USB cable/port
- Install device-specific USB drivers (Windows)
- Run `adb kill-server && adb start-server`

### App Crashes on Launch
- Check logcat for errors: `adb logcat | grep -E "AndroidRuntime|taurim"`
- Verify all permissions in AndroidManifest.xml
- Ensure minimum Android version compatibility

## APK Signing

### Creating a Keystore

Generate a keystore for signing release APKs:

```bash
keytool -genkey -v -keystore release.keystore -alias taurim -keyalg RSA -keysize 2048 -validity 10000
```

You will be prompted for:
- Keystore password
- Key alias (use `taurim` or your preferred name)
- Key password
- Distinguished name information (name, organization, etc.)

Keep the keystore file and passwords secure. Loss of the keystore means you cannot update existing app installations.

### GitHub Actions Secrets

For CI/CD releases, configure these secrets in **Settings > Secrets and variables > Actions > Environment secrets** (environment: `deployment`):

| Secret | Description |
|--------|-------------|
| `ANDROID_KEYSTORE_BASE64` | Base64-encoded keystore file: `base64 -w 0 release.keystore` |
| `ANDROID_KEYSTORE_PASSWORD` | Password for the keystore |
| `ANDROID_KEY_ALIAS` | Alias used when creating the key (e.g., `taurim`) |
| `ANDROID_KEY_PASSWORD` | Password for the key |

## CI/CD

### Automated APK Release

When a GitHub release is created, the workflow (`.github/workflows/20_taurim.yaml`) automatically:

1. Builds a signed APK for all Android architectures
2. Uploads the APK to the release as `taurim_{version}_android.apk`

To create a release:

```bash
gh release create v1.0.0 --title "v1.0.0" --notes "Release notes here"
```

The APK will be available for download from the release page after the workflow completes.

## License

MIT
