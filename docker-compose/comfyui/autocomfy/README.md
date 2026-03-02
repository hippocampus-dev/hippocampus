# autocomfy

<!-- TOC -->
* [Features](#features)
* [Usage](#usage)
* [Development](#development)
<!-- TOC -->

autocomfy is a web UI for automated ComfyUI workflow execution with result browsing and labeling.

## Features

- Single-page dashboard with run controls and result gallery side-by-side (responsive 2-column layout)
- Select and auto-execute ComfyUI workflows in infinite or fixed-count mode
- Real-time progress tracking via WebSocket with auto-refreshing results on completion
- Browse generated outputs (images, videos, audio) in a paginated gallery view
- Filter results by workflow topology hash to show only outputs from the selected workflow structure
- Label results as good/bad with optional reason for dataset curation
- View generation prompts (CLIPTextEncode node contents) alongside results in the label dialog
- Filter results by label status (all, unlabeled, good, bad)
- Automatic prompt generation via OpenAI-compatible LLM API using a concept description and good-labeled examples as references

## Usage

Runs as part of the `comfyui` Docker Compose profile at http://autocomfy.127.0.0.1.nip.io (via Envoy).

```sh
$ docker-compose --profile=comfyui up
```

## Development

```sh
$ cp .env.example .env
$ vi .env  # Set COMFYUI_URL and optionally OPENAI_BASE_URL, OPENAI_API_KEY, OPENAI_MODEL
$ make dev
```
