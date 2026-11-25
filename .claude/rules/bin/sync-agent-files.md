---
paths:
  - "bin/sync-agent-files.sh"
---

* Put a prohibition that must also bind Claude Code in `files/home/kai/.config/claudex/config/CLAUDE*.md` rather than in this script's `git_restriction` - that block is concatenated onto the Codex, Gemini, Antigravity and opencode outputs alone and never onto the `CLAUDE*.md` Claude Code reads, so one written there leaves Claude Code unbound while the script, its outputs and a grep for the prohibition all report it present
