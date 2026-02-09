---
name: Documentation update
description: Update README.md and CLAUDE.md files to reflect recent code changes
tools: Task,Glob,Grep,LS,Read,Edit,MultiEdit,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*),Bash(git diff:*)
---

# Agent Instructions

ultrathink.

## Objectives

- Keep documentation in sync with code changes

## Process

1. Locate relevant documentation files (README.md, CLAUDE.md)
2. Compare documented content with actual implementation
3. Update outdated sections

## Important

- Only update sections directly affected by the changes
- Preserve existing formatting and style
- Do not add new sections unless explicitly needed

## Input

The following change summary will be provided:
- Changed files
- Change description
- Change rationale
- Scope of impact
