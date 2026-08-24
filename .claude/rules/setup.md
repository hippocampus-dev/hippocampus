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
* Put a tool this repository's own content invokes - a `bin/` script, a `.claude/skills/` pipeline - at every provisioning site it can run from, choosing the site by what invokes it rather than by the group its name would land in (`## Choosing the provisioning site`) - `setup.sh` picks `setup/arch/install-packages.sh` or `setup/ubuntu/install-packages.sh` off `/etc/os-release` while `.devcontainer/postCreateCommand.sh` reaches only the latter, so a tool added to one side alone works everywhere except the host nobody thought to test
* Name a package that resolves on the release `.devcontainer/devcontainer.json` pins rather than the Arch name it happens to share - `apt-get install` aborts the whole transaction on a virtual package carrying more than one provider, which is what `netcat` became on `ubuntu-24.04` where `netcat-openbsd` and `netcat-traditional` both provide it, and that abort stops `.devcontainer/postCreateCommand.sh` before `setup/asdf.sh` and `setup/user.sh` run at all, while a virtual package left with a single provider still resolves and keeps `man` reaching `man-db`
* Pass a third-party installer its PATH opt-out where it has one (`--no-modify-path` for rustup, uv and deno) - `~/.bash_profile` and `~/.config/fish/config.fish` are symlinks into `files/home/kai/`, so a PATH line an installer appends to either one lands as a diff in this repository, and where the installer offers no such flag the only thing holding it back is `files/etc/profile.d/path.sh` already exporting the directory it installs into, which is what keeps `https://antigravity.google/cli/install.sh` from touching them

## Choosing the provisioning site

Take the first row that matches.

| The tool is | Where it goes |
|-------------|---------------|
| Reached only through a unit, device or kernel module one side alone provisions (`square-webcam.service` sits in `setup/arch/env.sh`'s `_USER_SERVICES` while `setup/ubuntu/env.sh` leaves that array empty) | That side's list alone, whatever the other side packages (`v4l-utils`, `v4l2loopback-dkms`, `inotify-tools`) |
| Packaged on both, and the two packages resolve to the same program (`.claude/rules/slides.md` names `imagemagick` as one that does not) | The same comment group in each `install-packages.sh` (`xclip`, `gifsicle`) |
| Packaged usably on one side only | That side's list, and a release binary after the other's `apt-get install` (`asciinema`, `agg`) |
| Packaged on neither | A release binary into `~/bin` from `setup/user.sh`, the one script both callers run as the user (`kubectl`, `skaffold`, `telepresence`) |

## Choosing the re-run form

| What the repository owns | Form |
|--------------------------|------|
| The whole content of a declaration (MCP server registration, keyring file, directory) | Overwrite: `mkdir -p`, `gpg --dearmor --yes`, `remove ... \|\| true` then `add` |
| Only that a secret or identity exists, the host owning its value (SSH key, GPG key) | Query guard: `[ ! -f ~/.ssh/github ]`, `if ! gpg --list-keys` |
| Only that an account, group or membership exists | Query guard: `id`, `getent group`, `groups \| grep` |
| Only one block of a file the host also owns (`~/.ssh/config`) | Query guard on a distinctive line: `grep -q` before appending |
| A checkout tracking upstream, the remote owning its content | Reconcile in place: `git remote get-url origin` then `set-url` or `add` and pull (`setup/asdf.sh`), or `[ -d "$dir" ]` then `git pull` else `git clone` (`setup/arch/install-packages.sh`) |
