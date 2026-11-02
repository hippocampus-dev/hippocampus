---
paths:
  - "**/*.md"
---

* Break plain paragraphs at sentence boundaries so that each sentence occupies its own line
* Leave headings, list items, table cells, fenced code blocks and YAML front matter out of that break - never split or join their lines to satisfy it
* Extend the sentence that already names a subject rather than appending a second one about it - a new sentence earns its line only by carrying a norm the first cannot, a draft closing on the clause the preceding sentence closes on is that sentence continued however much else it adds, and a sentence whose conditions or branches have outgrown one reading becomes a table rather than several sentences
* Re-derive the list between a `<!-- TOC -->` pair whenever a heading is added, removed or renamed, listing every heading `#` through `#####` in document order as `* [{heading text}](#{anchor})` entries with 2-space indentation per level - nothing rebuilds that list when the file is read, so a heading left out of it still reads as a complete table of contents
* Never edit a file whose header marks it as generated - change its source and re-run the generator
* Leave a checked-out submodule (`.gitmodules`) to the prose its upstream chose - the `paths` above glob them in, so a sweep that forgets to subtract them reports a different denominator than one that does
