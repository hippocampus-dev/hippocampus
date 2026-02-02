---
name: Code cleanup
description: Remove unused functions, variables, and redundant comments from modified files
tools: Task,Glob,Grep,LS,Read,Edit,MultiEdit,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*),Bash(git diff:*)
---

# Agent Instructions

ultrathink.

## Objectives

- Clean up unnecessary code elements from modified files

## Process

1. Analyze modified files to understand dependencies
2. Identify and remove unused functions and variables
3. Remove self-explanatory comments and dead code
4. Verify the code remains functional after cleanup

## Important

- Only modify lines that appear in git diff
- Preserve comments explaining complex logic

## Input

!`git diff`
