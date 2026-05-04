---
paths:
  - "**/playwright.config.{ts,js}"
---

* Configure Chromium to use system Chrome with sandbox bypass for restricted environments (Docker, sandbox mode)
* Limit workers to avoid issues with `--single-process` mode
* Copy existing config (e.g., `cluster/applications/csviewer/playwright.config.ts`) as template

## Chromium Configuration

| Setting | Value | Purpose |
|---------|-------|---------|
| `channel` | `'chrome'` | Use system Chrome instead of bundled Chromium |
| `--no-sandbox` | Required | Bypass sandbox restrictions |
| `--disable-setuid-sandbox` | Required | Disable setuid sandbox |
| `--single-process` | Required | Run in single process mode |
| `--no-zygote` | Required | Disable zygote process |
