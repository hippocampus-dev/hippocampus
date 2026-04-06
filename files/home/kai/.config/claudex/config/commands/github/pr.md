---
description: Create pull request with appropriate title
allowed-tools: Bash(gh:*), Bash(git:*)
---

Create a pull request with an appropriate title based on the changes made.

## Branch and Changes

!`git branch --show-current`

!`git status --porcelain`

!`git diff --stat`

## Recent Commits

!`git log --oneline -n 10`

Create the pull request using `gh pr create` with an appropriate title and body following Linux kernel patch guidelines:

## PR Body Format (Linux Kernel Style)

The body should follow Linux kernel patch guidelines and include:

1. **User-Visible Impact**
   - Explain how this affects users or the system
   - Include specific circumstances that trigger the issue
   - Describe any security implications

2. **Solution Details**
   - Technical explanation in plain English
   - How the code change addresses the problem
   - For optimizations: include numerical evidence of improvements

3. **Trade-offs and Side Effects**
   - Potential downsides or performance impacts
   - Any compatibility concerns
   - Changes in behavior that users might notice

Format requirements:
- Use imperative mood ("Fix bug" not "Fixed bug")
- Write for permanent changelog - clear to future readers

Use `--fill` option to start, then enhance with these specific details.
