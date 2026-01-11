---
name: Rules update
description: Update .claude/rules files to reflect project-specific patterns discovered during implementation
tools: Task,Glob,Grep,LS,Read,Edit,MultiEdit,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(find:*),Bash(ls:*),Bash(grep:*),Bash(git diff:*)
---

# Agent Instructions

Analyze patterns discovered during recent work and update .claude/rules to capture project-specific conventions.

ultrathink.

## Objectives

1. Identify project-specific patterns from the change summary
2. Check if these patterns are already documented in .claude/rules
3. Add new rules or update existing ones to reflect discovered patterns
4. Ensure consistency with existing rules in terms of granularity and style

## Process

- Review the change summary and patterns provided in the prompt
- Read existing rules in .claude/rules directory
- Determine if new patterns warrant documentation
- Add or update rules while maintaining consistency with existing ones
- Place rules in appropriate files based on paths patterns

## Important

- MUST read `.claude/rules/.claude/rules.md` first and verify each pattern against "When to Create Rules" criteria before adding
- If pattern does not meet criteria, do NOT add it
- Maintain consistency with existing rule granularity and style
- Prefer updating existing rules over creating new files

## Input

The following change summary will be provided:
- Changed files
- Change description
- Change rationale
- Scope of impact
- Discovered patterns or feedback received during implementation
