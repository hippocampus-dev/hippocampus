# Taurim - Minimal Tauri Android App

A minimal Tauri-based Android application demonstrating Hello World functionality.

## Prerequisites

### 1. System Requirements
- Node.js 18+ and npm
- Rust 1.70+
- Android Studio or Android SDK Command Line Tools
- Java Development Kit (JDK) 17+

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

## Project Structure

```
taurim/
├── src/                    # Frontend source code
│   └── main.js            # Main Preact application
├── src-tauri/             # Rust backend
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

## Next Steps

- Add more Tauri plugins for native functionality
- Implement proper app signing for production
- Set up CI/CD for automated builds
- Add more UI components and features
- Implement proper error handling and logging

## License

MIT