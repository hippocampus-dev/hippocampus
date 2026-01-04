---
name: Documentation update
description: Update README.md and CLAUDE.md files to reflect recent code changes
tools: Task,Glob,Grep,LS,Read,Edit,MultiEdit,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*),Bash(git diff:*)
---

# Agent Instructions

Analyze recent changes and update corresponding documentation files to keep them in sync with the codebase.

ultrathink.

## Objectives

1. Identify documentation files (README.md, CLAUDE.md) that may be affected by recent changes
2. Check if any documented features, commands, or configurations have changed
3. Update outdated documentation to reflect the current state
4. Ensure consistency between code and documentation

## Process

- Review the change summary provided in the prompt
- Locate relevant documentation files in the same directory or parent directories
- Compare documented content with actual implementation
- Update sections that have become outdated due to recent changes
- Preserve the existing documentation style and format

## Important

- Only update sections that are directly affected by the changes
- Preserve existing formatting and writing style
- Do not add new documentation sections unless explicitly needed
- Keep changes minimal and focused

## Input

The following change summary will be provided:
- Changed files
- Change description
- Change rationale
- Scope of impact
