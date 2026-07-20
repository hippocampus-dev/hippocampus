# armyknife

<!-- TOC -->
* [armyknife](#armyknife)
  * [Usage](#usage)
    * [remotty](#remotty)
  * [Development](#development)
<!-- TOC -->

armyknife is a Go CLI tool that provides various utilities including container registry management and credential handling.

## Usage

### remotty

Connect to a remotty workspace via chisel reverse tunnel. Additional `REMOTE` arguments are passed directly to chisel; use explicit `127.0.0.1` binds to avoid listening on every interface.

```sh
$ armyknife remotty --auth nonroot:<password> https://remotty.kaidotio.dev/tunnel
$ armyknife remotty --auth nonroot:<password> https://remotty.kaidotio.dev/tunnel R:<remote-port>:127.0.0.1:<local-port>
$ armyknife remotty --auth nonroot:<password> https://remotty.kaidotio.dev/tunnel 127.0.0.1:<local-port>:<remote-host>:<remote-port>
$ armyknife remotty --auth nonroot:<password> --env LANG,LC_*,AWS_PROFILE https://remotty.kaidotio.dev/tunnel
```

`--env` accepts a comma-separated glob allow list evaluated with `path.Match` semantics (e.g. `LC_*`). Matching variables are exposed via the host bridge and sourced by the workspace shell on startup; run `remotty-env` inside an existing shell to refresh. Clipboard is relayed to the browser via OSC52 through tmux and ttyd.

## Development

```sh
$ make dev
```
