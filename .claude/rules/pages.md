---
paths:
  - "pages/**"
---

* Cloudflare Pages static site with API functions
* To expose files from other directories, use symlinks with relative paths (e.g., `ln -s ../../extension/manifest.json pages/extension/manifest.json`)

## Static File Serving

| Scenario | Method |
|----------|--------|
| Pages-specific content | Create file directly in `pages/` |
| Expose existing file | Create symlink to original file |

## Files

| File | Purpose |
|------|---------|
| `_redirects` | URL redirect/rewrite rules |
| `*.html` | Static HTML pages |
| `functions/` | Cloudflare Pages Functions (API) |
| `src/` | TypeScript source for functions |
