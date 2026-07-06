---
paths:
  - "files/**"
---

* Symlink source files managed by `setup.sh` - changes here require `setup.sh` updates
* `setup.sh` creates symlinks from `files/{target}` to `{target}` (e.g., `files/home/kai/.gitconfig` → `/home/kai/.gitconfig`)
* Adding, renaming, or deleting files MUST update `_USER_TARGETS` or `_SYSTEM_TARGETS` array in `setup.sh`

## Script Placement

| Script type | Location |
|-------------|----------|
| Project-dependent operations (repository management, CI/CD tasks) | `bin/` |
| Project-independent tools (system utilities, service integrations) | `files/usr/local/bin/` |

## Script Naming in `files/usr/local/bin/`

| Type | Naming | Example |
|------|--------|---------|
| Keybind-callable scripts | No extension, hyphen-separated | `screenshot`, `up-volume`, `toggle-easyeffects` |
| Utility scripts | `.sh` extension, hyphen-separated | `backup.sh`, `startup.sh`, `sync.sh` |

## Workflow

| Operation | `setup.sh` array | Suggest to user |
|-----------|------------------|-----------------|
| Add file | Add target path to `_USER_TARGETS` or `_SYSTEM_TARGETS` | `ln -s <source> <destination>` (per user in `_USERS` for `_USER_TARGETS`) |
| Rename file | Update entry | `rm <old_destination>` + `ln -s <new_source> <new_destination>` |
| Delete file | Remove entry | `rm <destination>` |

Source/destination follow `Directory Structure`.

## Directory Structure

| Path Pattern | Symlink Target |
|--------------|----------------|
| `files/home/kai/*` | `/home/kai/*` |
| `files/etc/*` | `/etc/*` |
| `files/usr/local/bin/*` | `/usr/local/bin/*` |
| `files/usr/share/*` | `/usr/share/*` |
