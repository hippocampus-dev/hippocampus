.DEFAULT_GOAL := dev

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: install
install:
	@npm ci

.PHONY: dev
dev: install
	@npm run dev

.PHONY: build
build: install
	@npm run build

.PHONY: android-init
android-init:
	@npm run tauri android init

.PHONY: android-dev
android-dev:
	@npm run tauri android dev

.PHONY: android-build
android-build:
	@npm run tauri android build

.PHONY: android-log
android-log:
	@$(ANDROID_HOME)/platform-tools/adb logcat -c && \
	$(ANDROID_HOME)/platform-tools/adb logcat "*:E" "RustStdoutStderr:V" "Tauri:V" "GeminiPlugin:V" "TimerPlugin:V" "TimerService:V"

.PHONY: FORCE
FORCE:
