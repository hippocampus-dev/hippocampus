.DEFAULT_GOAL := dev

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: fmt
fmt:
	@npm run fmt

.PHONY: lint
lint:
	@npm run lint

.PHONY: install
install:
	@npm ci

.PHONY: build
build:
	@npm run build

.PHONY: all
all: fmt lint install build
	@

.PHONY: dev
dev: install
	@npm run dev

.PHONY: android-init
android-init:
	@npm run tauri android init
	@sed -i 's/kotlin-gradle-plugin:[0-9]\+\.[0-9]\+\.[0-9]\+/kotlin-gradle-plugin:2.1.0/' src-tauri/gen/android/build.gradle.kts

.PHONY: android-dev
android-dev:
	@npm run tauri android dev

.PHONY: android-build
android-build:
	@npm run tauri android build

.PHONY: android-log
android-log:
	@$(ANDROID_HOME)/platform-tools/adb logcat -c && \
	$(ANDROID_HOME)/platform-tools/adb logcat "*:E" "RustStdoutStderr:V" "Tauri:V" "GeminiPlugin:V" "TimerPlugin:V" "TimerService:V" "SpeechPlugin:V"

.PHONY: FORCE
FORCE:
