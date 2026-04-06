# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

spotify-mcp-server is a Go-based MCP (Model Context Protocol) server that provides read-only access to Spotify's track-related APIs. It implements the MCP authorization specification with Streamable HTTP transport, delegating OAuth authorization to an external oauth-bridge service.

## Common Development Commands

### Development
- `make dev` - Runs the service with auto-reload using watchexec
- `make all` - Run full suite: format, lint, tidy, test
- `make gen` - Regenerate Spotify API client from OpenAPI spec

### Configuration
Optional environment variables:
- `BASE_URL`: Public base URL of this server (default: `http://localhost:8080`)
- `AUTHORIZATION_SERVER`: OAuth authorization server URL (default: `https://hippocampus-dev-spotify-oauth-bridge.kaidotio.dev`)

## Architecture

### OAuth Authorization Flow
The server delegates OAuth authorization to an external oauth-bridge service. The Protected Resource Metadata (`/.well-known/oauth-protected-resource`) points MCP clients to the oauth-bridge for token acquisition. The server expects Spotify access tokens as Bearer tokens in the Authorization header.

### Endpoints
- `GET /.well-known/oauth-protected-resource` - Protected Resource Metadata (RFC 9728)
- `POST /mcp` - Streamable HTTP MCP endpoint
- `GET /healthz` - Health check

### MCP Tools (read-only)
- `search_tracks` - Search for tracks
- `get_track` - Get a single track by ID
- `get_several_tracks` - Get multiple tracks by IDs
- `get_saved_tracks` - Get user's saved tracks (requires user-library-read)
- `check_saved_tracks` - Check if tracks are saved (requires user-library-read)
- `get_album_tracks` - Get tracks from an album
- `get_user_top_tracks` - Get user's top tracks (requires user-top-read)
- `get_playlist_tracks` - Get tracks in a playlist
- `get_currently_playing` - Get currently playing track (requires user-read-currently-playing)
- `get_recently_played` - Get recently played tracks (requires user-read-recently-played)

### OpenAPI Client
The Spotify API client is auto-generated from the official OpenAPI spec using ogen (client-only mode). Generated code lives in `internal/spotify/`.
