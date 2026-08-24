---
paths:
  - "files/home/kai/.config/fish/**"
---

* Reach for `psub` where `## Secrets in argv` in `.claude/rules/bash.md` calls for a process substitution, as `-H @(printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN" | psub)` - fish has no `<(...)`, and `psub` hands over the path of a 0600 file rather than a pipe, in a directory it chooses itself, so that rule's `${XDG_RUNTIME_DIR:-/tmp}` bullet cannot reach it
* Hand the value straight to a fish builtin or function (`string replace`, `history --delete`) - those run inside the shell rather than as a separate process, so no `/proc/PID/cmdline` carries it, and routing them through `psub` instead breaks the ones that take a string where a file is not accepted
* Gate an `asdf install` on `~/.asdf/installs/{tool}/{version}`, reading the version out of `~/.tool-versions` with `string replace -rf "^$tool_name " "" < ~/.tool-versions` - asdf keeps each installed version in its own child directory, so a guard on `~/.asdf/installs/{tool}` alone stays true once any version is there and a bumped `~/.tool-versions` never reaches the install, while `asdf where` sees the version but puts a subprocess in front of every interactive shell
