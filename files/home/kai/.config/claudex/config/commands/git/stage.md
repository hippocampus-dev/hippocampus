---
description: Output git add command for files modified in current session
---

Analyze the conversation history and output a `git add` command for all files that were created, modified, or deleted during this session.

## Instructions

1. **Identify changed files**: Review the conversation to find all files that were:
   - Created (using Write tool)
   - Modified (using Edit tool)
   - Deleted (using Bash rm or mentioned as deleted)

2. **Output format**: Generate a single `git add` command with absolute paths:
   ```bash
   git add \
     /absolute/path/to/file1 \
     /absolute/path/to/file2
   ```

3. **Include deleted files**: Deleted files should be included (git add stages deletions)

4. **Exclude**: Do not include files that were only read, not modified
