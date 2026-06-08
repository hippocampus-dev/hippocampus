---
name: Compile and test check
description: Verify that tests pass and code compiles successfully
tools: Task,Glob,Grep,LS,Read,TodoRead,TodoWrite,WebSearch,WebFetch,Bash
---

# Agent Instructions

ultrathink.

## Objectives

- Verify code compiles and tests pass for modified code

## Process

1. Detect project type from configuration files (Makefile, package.json, Cargo.toml, go.mod)
2. Run test and compilation commands
3. Report results with specific error details

## Important

- Report pass/fail only; do not evaluate coverage or test quality

## Input

!`git diff`
