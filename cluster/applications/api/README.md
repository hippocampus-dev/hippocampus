# api

<!-- TOC -->
* [api](#api)
  * [Usage](#usage)
    * [Anthropic Messages API (`/v1/messages`)](#anthropic-messages-api-v1messages)
    * [Realtime API (`/v1/realtime`)](#realtime-api-v1realtime)
  * [Development](#development)
<!-- TOC -->

api is an AI-powered API service providing OpenAI-compatible endpoints with advanced agent capabilities.

## Usage

| Endpoint | Description |
|----------|-------------|
| `/v1/chat/completions` | OpenAI-compatible chat completions with optional Cortex mode for agent capabilities |
| `/v1/messages` | Anthropic Messages API compatible interface using OpenAI models |
| `/v1/responses` | OpenAI Responses API endpoint (proxy mode) |
| `/v1/realtime` | WebSocket endpoint for real-time chat |
| `/v1/images/generations` | Image generation endpoint |
| `/healthz` | Health check endpoint |
| `/metrics` | Prometheus metrics endpoint |

### Anthropic Messages API (`/v1/messages`)

The `/v1/messages` endpoint provides an Anthropic Messages API compatible interface that uses OpenAI models as the backend.
This allows clients using Anthropic SDK to interact with OpenAI models.

Features:
- Uses OpenAI model names (e.g., `gpt-5-mini`, `gpt-5.4`) instead of Claude model names
- Supports streaming and non-streaming responses
- Supports tool use/function calling with Anthropic format
- Request/response format follows Anthropic Messages API specification

### Realtime API (`/v1/realtime`)

The `/v1/realtime` endpoint proxies the OpenAI Realtime API over WebSocket without altering the protocol.

Connections carrying the same audio under the same model and session settings share one upstream session: the first one sends audio to OpenAI and the others replay its events at their own playback position.
Clients need no changes and cannot tell the two roles apart.

Sharing:
- Compares the loudness curve of each connection's incoming audio by cross-correlation, which recovers both whether the audio matches and how far apart the two positions are
- Requires the correlation peak to be sharp as well as high, so periodic audio such as music declines to match instead of aligning to the wrong beat
- Begins only once a connection has sent `realtime_group_window_seconds` of audio, so every connection runs its own upstream session until then
- Re-checks the alignment periodically and leaves the group once it no longer holds
- Switches a connection between the two roles only after the response in flight has finished, so neither joining nor leaving cuts a response short; a connection whose upstream produces no responses therefore keeps its own session, while alignment that keeps failing ends the sharing without that wait
- Applies only to `audio/pcm` input, and never to a connection ahead of the one sending audio to OpenAI
- Lets only one of two connections sitting at the same position give up its upstream session, so neither is left without a feeder
- Matches two positions no further apart than roughly a third of `realtime_group_window_seconds`, beyond which the same audio no longer correlates strongly enough

While a connection is sharing, only `session.update` and `input_audio_buffer.append` are acted on.
Every other client event, `response.create`, `response.cancel`, `conversation.item.create` and `conversation.item.truncate` among them, is dropped, so a client that relies on barge-in or on injecting non-audio input loses that for the duration.

## Development

```sh
$ export OPENAI_API_KEY=<YOUR OPENAI API KEY>
$ make dev
```
