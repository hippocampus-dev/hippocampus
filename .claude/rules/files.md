---
paths:
  - "files/**"
---

* Symlink source files managed by `setup.sh` - changes here require `setup.sh` updates
* `setup.sh` creates symlinks from `files/{target}` to `{target}` (e.g., `files/home/kai/.gitconfig` → `/home/kai/.gitconfig`)
* Adding, renaming, or deleting files MUST update `_TARGETS` array in `setup.sh`

## Workflow

| Operation | Action |
|-----------|--------|
| Add file | Add target path to `_TARGETS` array in `setup.sh` |
| Rename file | Update corresponding entry in `_TARGETS` array |
| Delete file | Remove corresponding entry from `_TARGETS` array |

## Directory Structure

| Path Pattern | Symlink Target |
|--------------|----------------|
| `files/home/kai/*` | `/home/kai/*` |
| `files/etc/*` | `/etc/*` |
| `files/usr/local/bin/*` | `/usr/local/bin/*` |
| `files/usr/share/*` | `/usr/share/*` |
