# pianify

<!-- TOC -->
* [pianify](#pianify)
  * [Features](#features)
  * [Usage](#usage)
  * [Development](#development)
<!-- TOC -->

pianify is a web UI for practising MusicXML scores with a falling-note view, a piano keyboard and synchronized sheet music.

## Features

- Pick a score from the mounted score directory, or upload a MusicXML file from the browser
- Render the sheet music with the current playback position highlighted
- Show falling notes above a piano keyboard that lights up the notes being played
- Synthesize playback in the browser with adjustable tempo, volume and mute
- Practise the right hand, the left hand, or both
- Set a checkpoint and replay from it

## Usage

Runs as part of the root Docker Compose stack at http://pianify.127.0.0.1.nip.io (via Envoy).

```sh
$ docker compose up
```

## Development

```sh
$ make dev
```
