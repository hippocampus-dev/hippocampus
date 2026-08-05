---
paths:
  - "slides/**"
---

* Treat a newline inside a slide paragraph as layout - Marp renders it as `<br />`, so the sentence-per-line bullet in `.claude/rules/markdown.md` does not apply here; break a line only where the rendered slide should break
* Copy `slides/19000101.md` as the template for a new deck and name it `YYYYMMDD.md` after the presentation date, keeping its front matter and title slide - the `marp:*` scripts glob `slides/[0-9]*.md` to keep the generated `slides/AGENTS.md` out of the build, so a deck whose name does not start with a digit is silently never built
* Build a deck through `make marp-dev`, `make marp-build` or `make marp-dist` at the repository root rather than a bare `npm run marp:*` - the root `.npmrc` sets `ignore-scripts=true`, so a bare `npm run` drops the `premarp:*` hook that strips the metadata off the JPEGs in `slides/images` without saying so, and those targets are the only local entry points carrying the `--no-ignore-scripts` that restores it - the hook resolves each symlink and rewrites the target, so a build edits `images/Kai.jpg` at the repository root, which is itself one of `.github/workflows/20_pages.yaml`'s `paths` triggers
* Strip a non-JPEG asset's metadata yourself before committing it to `slides/images` - the `premarp:*` hook matches `*.jpg` and `*.jpeg` case-insensitively and nothing else, so any other format reaches GitHub Pages carrying whatever it was committed with
* Reference every asset as `images/<name>` and keep that name within `[A-Za-z0-9._-]` - `bin/inline-slide-images.sh` walks only the `images/` directory beside the built HTML, so any other path stays an external reference in what `make marp-dist` produces, and markdown-it rewrites anything outside that set (a space to `%20`, an `&` to `&amp;`, a non-ASCII byte to its percent escape) until the reference no longer matches the file it names
