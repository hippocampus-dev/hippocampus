---
paths:
  - "files/home/kai/.config/claudex/config/commands/**/*.md"
---

* Investigate existing commands to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `files/home/kai/.config/claudex/config/commands/` so the tracked outputs under `files/home/kai/.codex/`, `files/home/kai/.gemini/`, and `files/home/kai/.config/opencode/` do not drift from their source
* Frontmatter requires `description` field
* Add `allowed-tools` only when external tools (Bash, etc.) are needed
* Add `disable-model-invocation: true` unless the model is meant to fire the command with nobody typing it - Claude Code lists a command without it among the skills the model invokes on its own, and `/git:diff` fired that way overwrites the `.diff-$TMUX_PANE` a parallel session is still annotating
* Use `!` syntax for inline bash to show current state (e.g., `!`git branch --show-current``)
* End procedural commands with `## Instructions` using numbered steps
* `description` survives every conversion and `argument-hint` survives in Codex; `allowed-tools` has no command-file equivalent
* Write a keyword an `argument-hint` expects verbatim bare and mark a value the caller replaces with its own words as `<topic>`, leaving `[]` for a slot that can be left out - https://code.claude.com/docs/en/slash-commands gives `[issue-number]` and `[filename] [format]` as its own examples, so a hint shaped after them holds a literal and a placeholder inside one pair of brackets and the `/` autocomplete menu, which shows the line as written, leaves the caller nothing to tell the two apart by (`files/home/kai/.config/claudex/config/commands/git/stage.md`, `files/home/kai/.config/claudex/config/commands/git/diff.md`)
* Inline bash written as `` !`cmd` `` is rewritten to the `!{cmd}` form Gemini and Antigravity share, and to an explicit run-command instruction for Codex
* `$ARGUMENTS` is rewritten to `{{args}}` for Gemini and Antigravity, and remains `$ARGUMENTS` for Codex and opencode
* Spell `$ARGUMENTS` only where the argument's value belongs and name the argument in prose everywhere else - Claude Code substitutes every occurrence and `bin/sync-agent-files.sh` rewrites them with a global `sed`, so one written as the label of a branch is replaced by the very value it was meant to label, and an invocation carrying no argument leaves that label empty; a command branching on the argument rather than interpolating it spells none at all and names it in prose (`git/branch.md`, `git/stage.md`)
* Reshape a command's output or its argument grammar only after checking what already parses it and what else already writes it - `files/home/kai/bin/claudex-with-worktree` invokes `/git:stage hunks` and its `stage_session_changes` cuts the plan out of the reply with a `sed` on `hunks: <selection> file: <path>`, so a renamed keyword or a reshaped line matches nothing, and since `sed` exits 0 on no match the script falls through to `No hunk selection reported`, leaves the merge unrun and offers to delete the worktree branch carrying every uncommitted change
* Register a file a command writes into the working directory in `files/home/kai/.gitignore`, which `core.excludesFile` resolves to, with an unanchored pattern - `/git:stage` and `/git:diff` both list untracked files with `git status --porcelain` and take one the session created as a change of its own, so an unregistered artifact stages itself, this repository's own `.gitignore` reaches none of the other repositories the same global command runs in, and `files/home/kai/bin/claudex-with-worktree` sets the session's working directory from `git rev-parse --show-prefix`, so a root-anchored pattern misses the artifact whenever the session starts below the repository root
* Give such a file no extension a repository rule or sweep globs - `**/*.sh` takes a generated one-liner as a script to check and `**/*.md` takes a copied diff as prose to break at sentence boundaries (`files/home/kai/.config/claudex/config/commands/git/stage.md`, `files/home/kai/.config/claudex/config/commands/git/diff.md`)

## Command Types

| Type | Example | Structure |
|------|---------|-----------|
| Tool command | `git/weekly-report.md` | `allowed-tools` + `!` syntax + `## Instructions` |
| Persona command | `mimicry/kent-beck.md` | `# UPPERCASE HEADERS` + methodology + `# EXAMPLE WORKFLOW` |
| Simple delegation | `sop.md` | Description + single instruction |

## Reference

If creating a tool command:
  Read: `files/home/kai/.config/claudex/config/commands/git/weekly-report.md`

If creating a persona command:
  Read: `files/home/kai/.config/claudex/config/commands/mimicry/kent-beck.md`
