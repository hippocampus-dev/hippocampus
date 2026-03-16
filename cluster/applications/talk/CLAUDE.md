# CLAUDE.md

This file provides guidance to Claude Code when working with the Realtime Chat application.

## Project Overview

Realtime Chat (talk) is a pure frontend web application that enables real-time conversations with OpenAI's Realtime models. It uses WebSocket for direct connection to cortex-api and supports both text and audio inputs/outputs.

## Key Components

### Frontend (Static Web Application)
- `index.html`: Entry point with Tailwind CSS
- `components/App.js`: Main application component with:
  - WebSocket connection to cortex-api
  - Session configuration for the cortex-api proxy
  - Audio recording and streaming via MediaRecorder API
  - Real-time message exchange via WebSocket
  - Real-time transcription display
  - Audio playback using Web Audio API
- `components/App_webrtc.js`: Original WebRTC implementation (backup)

### Infrastructure
- `nginx.conf`: Nginx configuration for serving static files
- `Dockerfile`: Container image using nginx to serve the application

## Development Commands

- `make serve` - Serve locally with Python HTTP server
- `make dev` - Run with file watching (notifies on changes)
- `make docker-build` - Build Docker image
- `make docker-run` - Run in Docker container

## Important Notes

1. The application uses OpenAI's Realtime API via WebSocket which requires:
   - Access to the cortex-api proxy
   - WebSocket connection to cortex-api proxy
   - MediaRecorder API for audio capture
   - Web Audio API for audio playback
   - Proper session configuration for voice activity detection

2. Authentication flow:
   - The browser connects to cortex-api over WebSocket
   - cortex-api handles the authentication and proxies to OpenAI
   - No direct connection to OpenAI required

3. Browser requirements:
   - Microphone permissions must be granted
   - Modern browser with WebSocket support
   - HTTPS required in production for getUserMedia
   - Web Audio API support for audio playback

4. The frontend uses CDN-hosted Preact and Tailwind CSS for simplicity

## API Integration

1. **WebSocket Connection**: `wss://cortex-api.minikube.127.0.0.1.nip.io/realtime?model=gpt-realtime-mini`
2. **Authentication**: cortex-api attaches the upstream OpenAI credentials
3. **Message Protocol**: JSON messages following OpenAI Realtime API format
4. **Audio Format**: PCM16 at 24kHz sample rate for both input and output

## Common Issues

- Microphone permissions must be granted for audio recording
- The cortex-api proxy must be configured with valid OpenAI credentials
- CORS is handled by cortex-api proxy
- Audio must be converted from WebM to PCM16 format before sending
