---
description: Output hunk selections for files modified in current session
allowed-tools: Bash(git:*)
---

Analyze the conversation history and report which hunks of the working tree were created, modified, or deleted during this session.

## Working Tree Status

!`git status --porcelain`

## Instructions

1. **Read the working tree**: Use `git status --porcelain` to list untracked files and `git diff --no-textconv` to read the hunks of every tracked file.
   Hunks are numbered from 1 per file, in the order they appear in that file's diff.
   `--no-textconv` is required: a `.gitattributes` diff driver otherwise renumbers the hunks away from the ones staging walks.

2. **Identify changed files**: Review the conversation to find all files that were:
   - Created (using Write tool)
   - Modified (using Edit tool)
   - Deleted (using Bash rm or mentioned as deleted)

3. **Output format**: Emit one line per file and nothing else - no prose, no code fences:

       hunks: 1,3 file: /absolute/path/to/file1
       hunks: all file: /absolute/path/to/file2

   - `all` selects every hunk of that file
   - A comma-separated list of 1-based indexes selects individual hunks, written without spaces
   - The path comes last so that a path containing spaces stays parsable

4. **Report `all` where hunks do not apply**: Untracked files, deleted files, binary files, and mode-only changes carry no hunk the caller can address individually

5. **Include deleted files**: Deleted files should be included (staging them stages the deletion)

6. **Exclude**: Do not include files that were only read, not modified.
   Do not include hunks that were already present before this session started, even when they sit in a file this session also changed.
