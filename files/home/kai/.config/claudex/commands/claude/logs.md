---
description: Search past Claude Code conversation logs for previous work
allowed-tools: Bash(find:*), Bash(ls:*), Bash(stat:*), Bash(head:*), Bash(wc:*), Grep, Read
---

Search through past Claude Code conversation logs to find previous work, solutions, or discussions on a topic.

## Current Project

!`basename "$(pwd)"`

## Log Directory

!`echo "${HOME}/.config/claudex/projects/$(pwd | sed 's|/|-|g')"`

## Recent Log Files (last 5)

!`ls -lt "${HOME}/.config/claudex/projects/$(pwd | sed 's|/|-|g')"/*.jsonl 2>/dev/null | head -5`

## Instructions

1. **Interpret the query**: Understand what the user is looking for (time range, topic, specific file, command, etc.)

2. **Search**: Use Grep to find matching JSONL files, optionally filter by file modification time with `find`

3. **Read context**: Each JSONL line has `role` (user/assistant) and `content` fields

4. **Summarize**: Present relevant findings with date, context, and outcome
