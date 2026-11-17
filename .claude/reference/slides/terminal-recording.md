# Terminal Recording in a Deck

How a recorded terminal session gets into a Marp deck.
Producing the `.cast`, the GIF and the MP4 belongs to the `terminal-recording` skill; this file covers only what the deck build does with them.

Reference a GIF from the deck like any other asset:

```markdown
![](images/demo.gif)
```

Embed an MP4 as raw HTML, which the `marp:*` scripts allow through `--html`:

```html
<video src="images/demo.mp4" autoplay loop muted playsinline></video>
```

`make marp-dist` base64-encodes every asset into the HTML, adding about a third on top, so a long session is worth carrying as an MP4 rather than a GIF.

## What the Build Already Does

| Step | Where |
|------|-------|
| Strips the GIF's comment, name and application extensions, and re-optimizes its frames with `-O2` | `bin/strip-slide-image-metadata.sh`, which every `premarp:*` calls |
| Inlines the asset into the HTML as a `data:` URI | `bin/inline-slide-images.sh`, on `make marp-dist` only |

`bin/inline-slide-images.sh` resolves each asset's type with `file -bL --mime-type` rather than by extension, so a GIF and an MP4 both inline without changing it.
Its closing guard scans for a leftover `src="images/`, `url("images/` or `url(&quot;images/`, which the `<video>` form above satisfies.

## What It Does Not

An MP4 goes out carrying whatever `ffmpeg` wrote into it - see the metadata bullet in `.claude/rules/slides.md` for which formats the hook covers.
