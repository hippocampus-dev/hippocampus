---
paths:
  - "slides/**"
  - "package.json"
  - ".github/workflows/20_pages.yaml"
---

* Treat a newline inside a slide paragraph as layout - Marp renders it as `<br />`, so the sentence-per-line bullet in `.claude/rules/markdown.md` does not apply here; break a line only where the rendered slide should break
* Re-wrap a long source line yourself before pasting it into a fenced block, the one place the no-wrap bullet in `files/home/kai/.config/claudex/config/CLAUDE.general.md` does not reach - the `uncover` theme every deck inherits from `slides/19000101.md` routes each `pre` through marp-core's auto-scaling element, which holds the block at `width: max-content` and scales it down until its widest line fits instead of clipping it, so one long line shrinks every other line of that block with it and neither `make marp-build` nor the HTML it emits reports anything
* Copy `slides/19000101.md` as the template for a new deck and name it `YYYYMMDD.md` after the presentation date, keeping its front matter and title slide - the `marp:*` scripts glob `slides/[0-9]*.md` to keep the generated `slides/AGENTS.md` out of the build, so a deck whose name does not start with a digit is silently never built
* Build a deck through `make marp-dev`, `make marp-build` or `make marp-dist` at the repository root rather than a bare `npm run marp:*` - the root `.npmrc` sets `ignore-scripts=true`, so a bare `npm run` drops the `premarp:*` hook that strips the metadata off the JPEGs and GIFs in `slides/images` without saying so, and those targets are the only local entry points carrying the `--no-ignore-scripts` that restores it - the hook resolves each symlink and rewrites the target, so a build edits `images/Kai.jpg` at the repository root, which is itself one of `.github/workflows/20_pages.yaml`'s `paths` triggers
* Strip the metadata off an asset `bin/strip-slide-image-metadata.sh` does not cover yourself before committing it to `slides/images` - every `premarp:*` hook calls that one script and it matches `*.jpg`, `*.jpeg` and `*.gif` case-insensitively and nothing else, so any other format reaches GitHub Pages carrying whatever it was committed with
* Add a tool `bin/strip-slide-image-metadata.sh` calls to the apt line in `.github/workflows/20_pages.yaml` on top of the sites `.claude/rules/setup.md`'s `## Choosing the provisioning site` covers - that line installs only what it names and no `paths` entry there reaches this file, so the deck otherwise builds everywhere except the Pages deploy
* Pick a tool that resolves to the same program on Arch and on `ubuntu-24.04` - `imagemagick` does not, since Arch ships ImageMagick 7 and Ubuntu 24.04 ships 6, so only the former carries `magick` while the `convert` both provide is a symlink to it on one side and the real, differently-behaved binary on the other
* Reference every asset as `images/<name>` and keep that name within `[A-Za-z0-9._-]` - `bin/inline-slide-images.sh` walks only the `images/` directory beside the built HTML, so any other path stays an external reference in what `make marp-dist` produces, and markdown-it rewrites anything outside that set (a space to `%20`, an `&` to `&amp;`, a non-ASCII byte to its percent escape) until the reference no longer matches the file it names

## Reference

If putting a terminal recording into a deck:
  Read: `.claude/reference/slides/terminal-recording.md`
