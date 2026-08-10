---
paths:
  - "setup.sh"
  - "setup/**/*.sh"
  - ".devcontainer/postCreateCommand.sh"
---

* Every command that creates host state must survive a second run, either by a query that already succeeds once the state exists (`id "$user" || useradd`, `getent group || groupadd`) or by a form that overwrites what is there (`mkdir -p`, `gpg --dearmor --yes`, `claude mcp remove ... > /dev/null 2>&1 || true` ahead of `claude mcp add`) - these scripts are run again on an already-provisioned host, and a tool that exits non-zero on "already exists" aborts the whole run under the `set -Eeo pipefail` that `.claude/rules/bash.md` requires, skipping every later step including `setup.sh`'s closing `reboot`
* The `rmdir` before `ln -s` in `setup.sh`'s target loops is the one deliberate exception, since aborting is how a destination holding anything the repository cannot reproduce gets noticed - `.claude/rules/files.md`'s `## Workflow` owns that decision and prescribes the manual `rm -rf`, and its `paths` never reach `setup.sh`, so the bullet above is not licence to convert it
* Choose between the two forms by what the repository owns - the overwriting form where it declares the whole content of the state, the query guard where it declares only that the state exists and the host owns what is inside, so an MCP registration or a keyring file is rewritten every run while `setup/user.sh`'s SSH and GPG keys are left alone and `~/.ssh/config` gains only its own block
* Reach for `|| true` only where the sole expected failure is "not there yet", such as the `claude mcp remove` that precedes an add - anywhere else it swallows the failure that has to stop the run

## Choosing the re-run form

| What the repository owns | Form |
|--------------------------|------|
| The whole content of a declaration (MCP server registration, keyring file, directory) | Overwrite: `mkdir -p`, `gpg --dearmor --yes`, `remove ... \|\| true` then `add` |
| Only that a secret or identity exists, the host owning its value (SSH key, GPG key) | Query guard: `[ ! -f ~/.ssh/github ]`, `if ! gpg --list-keys` |
| Only that an account, group or membership exists | Query guard: `id`, `getent group`, `groups \| grep` |
| Only one block of a file the host also owns (`~/.ssh/config`) | Query guard on a distinctive line: `grep -q` before appending |
| A checkout tracking upstream, the remote owning its content | Reconcile in place: `git remote get-url origin` then `set-url` or `add` and pull (`setup/asdf.sh`), or `[ -d "$dir" ]` then `git pull` else `git clone` (`setup/arch/install-packages.sh`) |
