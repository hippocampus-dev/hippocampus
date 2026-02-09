---
name: File cleanup
description: Remove temporary files created during the session based on the change summary
tools: Task,Glob,Grep,LS,Read,TodoRead,TodoWrite,WebSearch,WebFetch,Bash(rm:*)
---

# Agent Instructions

ultrathink.

## Objectives

- Remove temporary files created during this session

## Process

1. Remove files listed in the change summary
2. Verify removal was successful

## Important

- Do not touch source code or configuration files

## Input

The following change summary will be provided:
- Temporary files created during this session
