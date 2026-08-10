# Terminal Recording in a Deck

How to get a terminal session into a Marp deck as an animated asset.

## Pipeline

```bash
asciinema rec demo.cast
agg demo.cast slides/images/demo.gif
```

Reference it from the deck like any other asset:

```markdown
![](images/demo.gif)
```

`asciinema` defaults to asciicast-v3 whatever the file is named, and only a path ending in `.txt` selects another format.
End the session with `<ctrl+d>` or `exit`, and pause capture with `<ctrl+\>`.
`agg` reads asciicast v1, v2 and v3 and encodes through gifski, so no intermediate step is needed.
It arrives through the `asciinema-agg.git` entry in `setup/arch/install-packages.sh`'s `_AURS`, which the package name does not give away.

`asciinema` refuses to overwrite an existing file, so re-recording over the same path needs `--overwrite`.

Both steps run on Arch only.
`setup/ubuntu/install-packages.sh` provisions neither `asciinema` nor `agg` nor `ffmpeg`, so nothing in this file works from the devcontainer that `.devcontainer/postCreateCommand.sh` builds.

Never commit the `.cast`.
The GIF beside it already carries the same session, and a second copy drifts from it the moment either is regenerated.
No `.gitignore` covers `*.cast`, so one left behind shows up as untracked in every later `git status`.
Write it under a directory the repository already ignores rather than at the root - `kernel-lab/.gitignore` ignores `/.build/`, and the root `.gitignore` names no scratch path at all - so forgetting one costs nothing.

## Recording From Inside the Sandbox

`asciinema` records a pseudo-terminal, and `files/home/kai/bin/claudex` grants the sandbox one through its `/dev/ptmx` and `char-pts` `DeviceAllow` entries, so a recording runs entirely inside the sandbox with no tmux involved.
Use the same pipeline as above; the `.cast` lands in the repository working tree, which the sandbox writes and `agg` reads.

Only what `-c` runs gets captured, so a session worth watching because someone is typing through it cannot be produced this way; drive an interactive recording from a real terminal on the host instead.

The `DeviceAllow` grant is validated on the host, since the sandbox cannot mount a devpts to test it.
If `openpty` still fails with `out of pty devices`, the sandbox devpts is reaching the recording code with `ptmxmode=000`, and recording stays unavailable until that grant is corrected on the host.

## MP4

A GIF of a long session grows fast, and `make marp-dist` base64-encodes every asset into the HTML, adding about a third on top.
Convert to MP4 when that matters:

```bash
ffmpeg -i slides/images/demo.gif -vf "scale=trunc(iw/2)*2:trunc(ih/2)*2" -map_metadata -1 -fflags +bitexact -flags:v +bitexact -movflags +faststart -pix_fmt yuv420p slides/images/demo.mp4
```

The `scale` filter is not optional.
libx264 rejects an odd width or height outright (`width not divisible by 2`), and a terminal geometry lands on one often enough to matter.

The `-map_metadata` and `bitexact` flags drop the container's `encoder` tag and cut the stream's down to `Lavc libx264`.
`handler_name` and `language` survive them, so an MP4 never comes out as clean as what the hook does to a GIF.

Embed it as raw HTML, which the `marp:*` scripts allow through `--html`:

```html
<video src="images/demo.mp4" autoplay loop muted playsinline></video>
```

## What the Build Already Does

| Step | Where |
|------|-------|
| Strips the GIF's comment, name and application extensions, and re-optimizes its frames with `-O2` | `bin/strip-slide-image-metadata.sh`, which every `premarp:*` calls |
| Inlines the asset into the HTML as a `data:` URI | `bin/inline-slide-images.sh`, on `make marp-dist` only |

`bin/inline-slide-images.sh` resolves each asset's type with `file -bL --mime-type` rather than by extension, so a GIF and an MP4 both inline without changing it.
Its closing guard scans for a leftover `src="images/`, `url("images/` or `url(&quot;images/`, which the `<video>` form above satisfies.

## What It Does Not

An MP4 goes out carrying whatever `ffmpeg` wrote into it - see the metadata bullet in `.claude/rules/slides.md` for which formats the hook covers.
