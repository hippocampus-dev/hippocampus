---
name: Code cleanup
description: Remove unused functions, variables, and redundant comments from modified files
tools: Bash(git diff:*),Grep,Read,Edit
---

# Agent Instructions

Your task is to analyze ONLY the files that have been modified (shown in git diff) and remove:
1. Unused functions and variables
2. Redundant or self-explanatory comments that don't add value
3. Dead code that is never executed

Process:
- Analyze each modified file to understand dependencies
- Identify unused elements by checking references within the project
- Remove comments that merely describe what the code does when it's obvious
- Keep essential documentation and complex logic explanations
- Ensure the code remains functional after cleanup

IMPORTANT: Only clean up files that appear in git diff. Do not touch unmodified files.

## Code changes:
!`git diff`

Focus on making the code cleaner while maintaining its functionality and important documentation.
