---
description: Write the hunks the current session changed into a diff file the user annotates with HTML comments, or read those annotations back and change the working tree, narrowed to one topic when the argument names one
argument-hint: [preview|feedback] [<topic>]
allowed-tools: Bash(git:*), Bash(echo:*), Read, Write, Edit
disable-model-invocation: true
---

Analyze the conversation history and either write the hunks of the working tree that were created, modified, or deleted during this session into a file the user can annotate, or act on the annotations that file already carries.

## Working Tree Status

!`git status --porcelain`

## Session Pane

!`echo $TMUX_PANE`

## Instructions

1. **Pick the form**: The argument after this prompt names the topic, apart from a leading `preview` or `feedback` that picks the form instead:

   | Argument | Emit |
   |----------|------|
   | a leading `preview`, or neither word | The diff file of step 4 and the report of step 5 |
   | a leading `feedback` | The working-tree changes of step 6 |

   - Both forms use `.diff-$TMUX_PANE` in the working directory, since parallel sessions share that directory and a fixed name lets one overwrite the diff of another
   - Do nothing and say so when `$TMUX_PANE` is empty, since every session running without one would otherwise share a single `.diff-` file

2. **Read the working tree**: Use `git status --porcelain` to list untracked files and `git diff --no-textconv` to read the hunks of every tracked file.
   Hunks are numbered from 1 per file, in the order they appear in that file's diff.
   `--no-textconv` is required: a `.gitattributes` diff driver otherwise renumbers the hunks away from the ones `/git:stage` walks, and a selection read out of this file has to name the same hunk that command does.

3. **Identify changed files**: Review the conversation to find all files that were:
   - Created (using Write tool)
   - Modified (using Edit tool)
   - Deleted (using Bash rm or mentioned as deleted)

4. **Write the diff file**:

   - A section is `hunks: 1,3 file: /absolute/path/to/file1` or `hunks: all file: /absolute/path/to/file2` on its own line, a blank line, then that file's selected hunks copied verbatim out of its diff, and sections are separated by a blank line
   - The header line carries the grammar `/git:stage hunks` prints, so that a selection read back out of this file reaches that command unchanged
   - The path comes last in a header line so that a path containing spaces stays parsable
   - `all` selects every hunk of that file, and a comma-separated list of 1-based indexes selects individual hunks, written without spaces
   - An untracked file takes a header line with `all` and a body from `git diff --no-textconv --no-index -- /dev/null '/absolute/path/to/file2'`, which reports the difference through exit status 1, since `git add -p` walks no hunk for a path it does not track and a numbered selection handed to `/git:stage` would stage nothing
   - A deleted, binary or mode-only file takes a header line with `all` and no hunk body, since none of them carries a hunk that can be shown as a diff
   - A file the topic leaves without a single selected hunk carries no section, and a topic that leaves no file at all emits one line saying nothing in this session relates to it and writes no file

5. **Report**: Where a diff file was written, emit one summary line per file it holds, then its absolute path, and nothing else:

   - A summary line is `- /absolute/path/to/file1: hunks 1,3 of 5 - 変更内容` or `- /absolute/path/to/file2: all - 変更内容`, is written in Japanese, names a mode change beside the hunks where the file carries one, and carries no reason for a hunk being in or out and none for the change itself
   - A summary line reports the selection the diff file encodes, since one written beside the file rather than from it can name a file or a hunk the file does not carry

6. **Act on the annotations**: Read the diff file and change the working tree to address every marker the user left in it:

   - A marker runs from a `<!--` at column 0 through the `-->` that closes it, over as many lines as it takes, and one sitting inside a diff line was copied out of the file under review rather than written by the user, since every line copied out of a diff carries a ` `, `+` or `-` prefix
   - A marker belongs to the hunk it sits in, and one sitting outside every hunk belongs to the file its section names
   - A marker is an instruction from the user rather than a finding, so it is not sorted through `### Feedback` and the answer-only rule `## Response Policy` gives a question does not reach it
   - Locate what a marker names in the working tree before changing anything, and ask rather than guess where the file has moved on since the diff was written or where the comment reads more than one way
   - Save the diff file with the markers that were addressed removed and the rest left in place, since a marker that could not be acted on is the only record that it is still open
   - Report what changed and name every marker still left in the file
   - Change nothing and say so where the diff file does not exist or carries no marker at column 0

7. **Exclude**: Do not include files that were only read, not modified.
   Do not include hunks that were already present before this session started, even when they sit in a file this session also changed, nor the files and hunks this session changed for something other than the topic when the argument names one.
   Judge that relation from what the conversation was doing when the edit was made, and from the hunk's own content where one exchange changed things for several topics.
