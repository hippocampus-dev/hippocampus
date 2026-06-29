# Parallel Execution Pattern

How to run commands in parallel with proper wait handling.

## Standard Template

```bash
pids=()
while IFS= read -r file; do
  (
    cd "$(dirname "$file")"
    some_command
  ) &
  pids+=($!)
done < <(find . -type f -name "*.ext")

for pid in "${pids[@]}"; do
  wait "$pid"
done
```

## Key Points

* Use process substitution `< <(...)` to avoid subshell (pipe runs while loop in subshell, losing `pids` array)
* `(...)` creates subshell for parallel execution
* `&` runs in background
* `$!` captures last background PID
* `pids+=($!)` collects all PIDs
* `wait "$pid"` ensures all complete before exit

## With Cleanup

```bash
cleanup() {
  for pid in "${pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  exit 0
}

trap cleanup EXIT
```
