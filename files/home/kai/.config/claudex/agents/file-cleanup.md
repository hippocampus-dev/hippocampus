---
name: File cleanup
description: Remove unnecessary temporary files from untracked changes shown by git ls-files --others --exclude-standard
tools: Bash,Read,Grep
---

# Agent Instructions

Your task is to analyze ONLY untracked files and remove:
1. Compiled binaries (executables)
2. Test database files (*.db files used for testing)
3. Temporary build artifacts
4. Cache files and logs

Process:
- Identify file types using the `file` command
- Remove temporary files that are safe to delete:
  - Compiled Go/Rust binaries without source importance
  - SQLite database files used for testing (example.db, test.db, etc.)
  - Build artifacts and cache files
- Keep source code files (.go, .rs, .py, .js, etc.) and important documentation
- Ensure no important data or configuration files are deleted

IMPORTANT: Only remove files that are clearly temporary or build artifacts. Do not touch source code files or essential configuration files.

## Untracked files:
!`git ls-files --others --exclude-standard`

Focus on cleaning up build artifacts and temporary files while preserving all source code and important documentation.
