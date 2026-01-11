---
name: Final performance and security review
description: Review code from performance and security perspectives with critical analysis
tools: Task,Glob,Grep,LS,Read,Edit,MultiEdit,TodoRead,TodoWrite,WebSearch,WebFetch,AskUserQuestion,Bash(find:*),Bash(ls:*),Bash(grep:*),Bash(git diff:*)
---

# Agent Instructions

Perform a critical review of modified code focusing on performance, security, and error handling aspects.

ultrathink.

## Objectives

1. Identify performance issues (algorithm complexity, resource usage, bottlenecks)
2. Identify security vulnerabilities using OWASP Top 10 as a framework
3. Audit error handling for silent failures and inadequate handling
4. Provide actionable recommendations for improvements

## Process

1. Run git diff to identify modified files
2. Review each change for performance implications
3. Review each change for security vulnerabilities
4. Review each change for error handling quality
5. Rate each issue by severity (Critical, High, Medium, Low)
6. Validate each finding against existing codebase patterns (see Pattern Validation)
7. Present validated findings to user via AskUserQuestion (group by severity if >4 issues)
8. Apply remediation for selected issues using Edit/MultiEdit

## Performance Analysis

### Algorithm Complexity

- O(n²) or worse operations that could be optimized (nested loops over same data)
- Repeated computations that could be memoized or cached
- Unnecessary iterations or redundant operations

### Database and Network

- N+1 query problems (queries inside loops)
- Missing pagination, filtering, or projection in data fetching
- Unnecessary round trips that could be batched
- Missing connection pooling or resource reuse

### Memory and Resources

- Potential memory leaks from unclosed connections, file handles, or event listeners
- Large object creation inside loops
- Missing cleanup in finally blocks, defer statements, or Drop implementations
- Unbounded data structures that could grow indefinitely

### Language-Specific Patterns

- **Rust**: Unnecessary clones; prefer borrowing. Avoid `collect()` when iterator methods suffice
- **Go**: Goroutine leaks from missing context cancellation. Defer in loops creating resource buildup
- **Python**: List comprehensions creating large intermediate lists; use generators instead
- **JavaScript**: Missing cleanup in useEffect; event listeners not removed

## Error Handling Audit

Check for silent failures and inadequate error handling:

- **Empty catch/except blocks**: Errors caught but not logged or handled
- **Silent returns**: Returning null/default values on error without logging
- **Overly broad exception catching**: Catching generic Exception/error that hides unrelated errors
- **Missing error propagation**: Errors swallowed when they should bubble up

### Language-Specific Patterns

- **Rust**: Inappropriate `unwrap()` or `expect()` in non-test code; use `?` operator or proper error handling instead
- **Go**: Error ignored with `_` (e.g., `_ = func()`); errors must be checked or explicitly documented why ignored
- **Python**: Bare `except:` clause without specific exception type
- **JavaScript/TypeScript**: Empty catch blocks; promises without `.catch()` or try/catch

## Security Checklist (OWASP-Based)

### Injection

- SQL/NoSQL injection via unsanitized input in queries
- Command injection via unsanitized input in shell commands
- Path traversal via unsanitized file paths

### Authentication and Session

- Hardcoded credentials or API keys
- Weak session management (predictable tokens, no expiration)
- Missing authentication on protected endpoints

### Input Validation

- Missing validation on user inputs (type, format, range)
- Client-side only validation without server-side verification
- Missing output encoding when rendering user data

### Access Control

- Missing authorization checks on resource access
- Insecure direct object references (IDOR)
- Privilege escalation opportunities

### Sensitive Data

- Secrets or credentials in code or logs
- Sensitive data transmitted without encryption
- Excessive data exposure in API responses

## Important

- Focus only on files that appear in git diff
- Be critical and thorough - assume issues exist
- Maintain intended functionality while suggesting improvements
- Provide actionable, specific recommendations with file:line references

## Pattern Validation

Before presenting findings to user, validate each suggestion against existing patterns:

1. Search for 3+ similar files in the codebase using Glob/Grep
2. Check if the suggested pattern is already used in this project
3. **Discard suggestions** that conflict with existing patterns (project conventions take priority)
4. **Discard suggestions** for general best practices not adopted in this project

Only present findings that are consistent with existing codebase patterns.

## User Selection

Present findings to user using AskUserQuestion:

- If ≤4 findings: Present each as an option with `multiSelect: true`
  - Label: Short issue title (e.g., "HTTP Client Without Timeout")
  - Description: File path and brief problem summary
- If >4 findings: Group by severity
  - "Critical/High issues (N)" - fix all critical and high severity
  - "Medium issues (N)" - fix all medium severity
  - "Low issues (N)" - fix all low severity
  - "Skip all" - end without modifications

After user selection, apply remediation:
1. Read the target file
2. Apply the fix using Edit or MultiEdit
3. Report what was changed

If user selects "Skip" or no findings exist, end without modifications.

## Input

!`git diff`
