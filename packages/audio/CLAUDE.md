# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Audio Package Overview

The `audio` package is a cross-platform audio capture library that records system audio (loopback capture) on Linux and Windows. It provides a unified API for capturing what's playing through the system speakers.

## Common Development Commands

From the workspace root (`/opt/hippocampus`):
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues  
- `cargo test` - Run tests
- `cargo udeps --all-targets --all-features` - Check for unused dependencies
- `cross build --target <target>` - Cross-compile for Linux targets
- `make all` - Run formatting, linting, testing, and build all targets

## High-Level Architecture

### Platform Abstraction
The package uses conditional compilation to provide platform-specific implementations:
- **Linux** (`linux.rs`): Uses PulseAudio for audio capture
- **Windows** (`windows.rs`): Uses Windows Audio Session API (WASAPI)
- **Fallback**: No-op implementations for unsupported platforms

### Core Components

1. **Audio Capture API** (`lib.rs`):
   - `prepare_loopback()` - Platform-specific setup (creates virtual sink on Linux)
   - `capture_device()` - Records audio using callback pattern
   - `CaptureControl` enum for flow control

2. **Audio Processing** (`lib.rs`):
   - `convert_channels()` - Mono/stereo conversion
   - `resample()` - Sample rate conversion with downsample/upsample algorithms
   - Efficient algorithms: mean-based downsampling, linear interpolation upsampling

3. **Utilities** (`utils.rs`):
   - PCM format conversion (bytes ↔ i16 samples)
   - Little-endian byte order handling

### Key Design Patterns

- **Callback-based capture**: Both platforms use `FnMut` callbacks for real-time audio processing
- **Resource safety**: Proper cleanup of system resources (COM, PulseAudio contexts)
- **Error propagation**: Custom error type with context information
- **Zero-copy where possible**: Direct buffer access on Windows, efficient conversions

### Platform-Specific Notes

**Linux**:
- Creates a PulseAudio null sink and loopback module
- Captures from the monitor source of the virtual sink
- Uses 1-second buffer size for stability

**Windows**:
- Direct loopback capture via WASAPI
- No preparation needed (prepare_loopback is no-op)
- Manages COM initialization/cleanup
- Uses system-determined buffer sizes