---
name: File cleanup
description: Remove unnecessary temporary files from untracked changes shown by git ls-files --others --exclude-standard
tools: Task,Glob,Grep,LS,Read,Edit,MultiEdit,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*),Bash(file:*),Bash(git ls-files:*),Bash(rm:*)
---

# Agent Instructions

Analyze untracked files and remove temporary or build artifacts while preserving source code and important files.

ultrathink.

## Objectives

1. Remove compiled binaries and executables
2. Remove test database files (*.db files used for testing)
3. Remove temporary build artifacts
4. Remove cache files and logs

## Process

- Run git ls-files --others --exclude-standard to list untracked files
- Identify file types using the `file` command
- Remove temporary files that are safe to delete
- Verify no important data or configuration files are deleted

## Important

- Only remove files that are clearly temporary or build artifacts
- Do not touch source code files (.go, .rs, .py, .js, etc.)
- Do not touch essential configuration files
- Preserve important documentation

## Input

!`git ls-files --others --exclude-standard`
