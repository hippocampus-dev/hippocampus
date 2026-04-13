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

| Operation | Action |
|-----------|--------|
| Add file | Add target path to `_USER_TARGETS` or `_SYSTEM_TARGETS` array in `setup.sh` |
| Rename file | Update corresponding entry in `_USER_TARGETS` or `_SYSTEM_TARGETS` array |
| Delete file | Remove corresponding entry from `_USER_TARGETS` or `_SYSTEM_TARGETS` array |

## Directory Structure

| Path Pattern | Symlink Target |
|--------------|----------------|
| `files/home/kai/*` | `/home/kai/*` |
| `files/etc/*` | `/etc/*` |
| `files/usr/local/bin/*` | `/usr/local/bin/*` |
| `files/usr/share/*` | `/usr/share/*` |
