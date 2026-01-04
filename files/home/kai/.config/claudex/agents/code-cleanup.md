---
name: Code cleanup
description: Remove unused functions, variables, and redundant comments from modified files
tools: Task,Glob,Grep,LS,Read,Edit,MultiEdit,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*),Bash(git diff:*)
---

# Agent Instructions

Analyze modified files shown in git diff and remove unnecessary code elements while preserving functionality.

ultrathink.

## Objectives

1. Remove unused functions and variables
2. Remove self-explanatory comments that don't add value
3. Remove dead code that is never executed

## Process

- Run git diff to identify modified files
- Analyze each modified file to understand dependencies
- Identify unused elements by checking references within the project
- Remove comments that merely describe what the code does when it's obvious
- Keep essential documentation and complex logic explanations
- Verify the code remains functional after cleanup

## Important

- Only clean up lines that appear in git diff
- Do not touch unmodified lines
- Preserve meaningful documentation and comments explaining complex logic

## Input

!`git diff`
