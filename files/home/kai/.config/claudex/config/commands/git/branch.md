---
description: Switch git branch with typo tolerance
allowed-tools: Bash(git:*)
---

Switch to a git branch with fuzzy matching for typo tolerance.

## Current State

!`git branch --show-current`

## Available Branches

!`git branch -a --format='%(refname:short)' | grep -v '^origin/HEAD' | sed 's|^origin/||' | sort -u`

## Instructions

1. **Parse the target branch**: Extract the branch name from the user's input after this prompt

2. **Fuzzy match**: If the exact branch doesn't exist, find the best match using these heuristics (in order of priority):
   - Exact match (case-insensitive)
   - Prefix match (e.g., "feat" matches "feature/login")
   - Contains match (e.g., "login" matches "feature/login")
   - Levenshtein distance for typos (e.g., "mian" matches "main", "mastr" matches "master")

3. **Confirm if ambiguous**: If multiple branches match with similar scores, ask the user to choose using AskUserQuestion

4. **Switch branch**:
   - If the branch exists locally: `git switch <branch>`
   - If only on remote: `git switch -c <branch> --track origin/<branch>`

5. **Report result**: Confirm which branch was switched to
