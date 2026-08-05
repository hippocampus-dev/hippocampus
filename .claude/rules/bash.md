---
paths:
  - "**/*.sh"
  - "files/usr/local/bin/*"
  - "files/home/kai/bin/*"
  - "files/home/kai/.asdf/plugins/*/bin/*"
  - "files/home/kai/llm/*"
---

* Use `#!/usr/bin/env bash` as shebang (`#!/usr/bin/env -S bash -l` for a login shell, `#!/usr/bin/env sh` for a script mounted into a container whose image ships no bash - confirm with `command -v bash` in that image rather than assuming) - see `## Header by script kind` for what carries neither shebang nor `set`
* Always include `set -Eeo pipefail` at the top - `-E` is what carries the next bullet's `ERR` trap into functions and command substitutions, which bash otherwise leaves silent, and it costs only a duplicated diagnostic when a `( )` subshell fails; see `## Header by script kind` for what takes less
* Add `trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR` after `set -Eeo pipefail` (skip wherever `## Header by script kind` reduces or drops `set`, and for POSIX sh - the `ERR` trap and `$BASH_COMMAND` are bash-only) - it stays quiet wherever a failure is already guarded, since any command but the last in an `&&`/`||` list, and an `if`, `while` or `until` condition, suppress it down into any command substitution they contain
* Use `[ ]` for conditionals (`[[ ]]` only when `<`, `>`, or `=~` is required)
* Use `-f` for files and `-d` for directories instead of `-e` (reserve `-e` for sockets, FIFOs, or type-agnostic checks)
* Quote variables with `"$var"` (use `"${var}"` when preceded or followed by adjacent characters like `"prefix${var}"` or `"${var}_suffix"`)
* Use `""` for strings by default, `''` only when shell expansion must be prevented (jq filters, regex patterns)
* For nested quotes (fzf --preview, etc.), use `""` for outer and `''` for inner jq filters
* Quote the heredoc delimiter (`<<'EOS'`) whenever the body carries a `$`, a backtick or a `\` that the consumer must receive literally, and leave it unquoted where the body needs shell expansion - where one body needs both, unquoted with each literal backslash-escaped is the only form that serves them together (`cluster/bin/initialize-vault.sh:10`) - an unquoted delimiter still processes `$` and `\`, so a `${...}` placeholder meant for a later renderer is substituted away before the consumer sees it, and a backslash escape written for the consumer collapses by one level (`\\` becomes `\`, which a YAML double-quoted scalar then rejects as an unknown escape)
* Use `while IFS= read -r` for file path processing (see subshell note below)
* Pass `-L` to `find` wherever the tree can hold symlinks - `setup.sh` and `pages/` expose assets that way by policy, and a symlink is `-type l`, so a bare `-type f` matches nothing, the loop body never runs and the script still exits 0 (`package.json`'s `premarp:*` scripts carry it because `slides/images` reaches `images/Kai.jpg` through a symlink)
* Remove the temp file on any failure wherever a step rewrites a tracked file through a new inode and renames it into place - `jpegtran` dying on a truncated JPEG leaves the part it had already written, and a `>` redirection creates the file before its writer runs at all, so even an immediate failure leaves an empty one beside the target, and no `.gitignore` entry covers either
* Copy the replaced file's mode onto the temp file (`chmod --reference`) before renaming it into place wherever the original's mode is what must survive - the `UMask=0077` sandbox that `.claude/rules/files.md` records leaves the fresh file at 0600 and the rename would commit that over the original's - while a generator writing into `files/` takes `.claude/rules/files.md`'s `umask 022` instead, since those outputs must land world-readable whatever mode the previous file carried
* Use `${XDG_RUNTIME_DIR:-/tmp}` for user-specific runtime files (credentials, session state, keys)
* Use `~` for home directory references (use `$HOME` in `${VAR:-default}` where `~` is not expanded)

## Header by script kind

Take the first row that matches, and leave a checked-out submodule (`.gitmodules`) or a `git check-ignore` hit to the header its upstream chose - the `paths` above glob them in, so a sweep that forgets to subtract them reports a different denominator than one that does.

| Script | Shebang | `set` line | Why |
|--------|---------|------------|-----|
| An `entrypoint.sh`, or a session-start script such as `files/etc/xrdp/startwm.sh` | `#!/usr/bin/env bash` | none | Aborting midway kills the very thing it exists to start |
| A `files/etc/profile.d/` fragment | none | none | `/etc/profile` sources them by glob rather than running them, and `errexit` would outlive the fragment in the login shell |
| A Kubernetes probe | `#!/usr/bin/env bash` | `set -e` | Its exit status is the health verdict the kubelet reads rather than an error to report |
| A sourced library that disclaims `set` and `trap` in its own header, such as `kernel-lab/lib.sh` | `#!/usr/bin/env bash` | none | The script that sources it owns the `set` line and the `ERR` trap |
| A script mounted into a container whose image ships no bash | `#!/usr/bin/env sh` | `set -e` | `pipefail` is not in POSIX and fails to parse in dash |
| An empty placeholder such as `.devcontainer/postStartCommand.sh` | none | none | There is nothing to guard until it has a body, and a header alone would claim otherwise |
| A generated `AGENTS.md` a `paths` entry globs into, such as `files/home/kai/llm/AGENTS.md` | none | none | Not a script - `bin/sync-agent-files.sh` generates it |
| Every other script | `#!/usr/bin/env bash`, or `#!/usr/bin/env -S bash -l` for a login shell | `set -Eeo pipefail` | Being sourced is not an exemption by itself - `setup/arch/env.sh` and `cluster/bin/export.sh` carry the full form, as do a container `command` that is not an `entrypoint.sh` (`cluster/manifests/utilities/redis/files/redis-server.sh`) and a script file under `.github/` |

## Variable naming

| Scope | Case | Examples |
|-------|------|----------|
| Environment / exported | UPPERCASE | `GITHUB_TOKEN`, `RUST_BACKTRACE` |
| Script-level constants | UPPERCASE | `REPOSITORY`, `ENTRYPOINT` |
| Function-local variables | lowercase | `message`, `default_branch` |
| Loop variables (in functions) | lowercase | `branch`, `secret`, `i` |

## Loop patterns

| Pattern | Variables | Loop errors | Command errors | Use when |
|---------|-----------|-------------|----------------|----------|
| `while read < <(cmd)` | Preserved | Yes | No | `find`, reliable commands |
| `var=$(cmd); for x in $var` | N/A | Yes | Yes (`pipefail`) | API calls, may-fail commands |
| `cmd \| while read` | Lost | No | Yes (`pipefail`) | Infinite streams (`tail -f`, `kubectl logs -f`) |

### Process substitution (for `find` etc.)

```bash
pids=()
while IFS= read -r file; do
  process "$file" &
  pids+=($!)
done < <(find . -type f -name "*.ext")
```

### Variable + for loop (for API calls)

When the command may fail (network, auth), use variable assignment to detect errors via `pipefail`:

```bash
branches=$(curl -fsSL ... | jq -re '.[].name')

for branch in ${branches}; do
  ...
done
```

### Pipe (for infinite streams)

For commands that never terminate, use pipe to detect abnormal exit via `pipefail`:

```bash
pactl subscribe | while IFS= read -r event; do
  ...
done
```

## xargs

Always use `-r` (--no-run-if-empty) to prevent command execution when input is empty:

| Condition | Pattern |
|-----------|---------|
| Multiple arguments supported | `xargs -r` |
| One-at-a-time processing | `xargs -r -L1` |
| Placeholder substitution | `xargs -r -I{}` |

`-L1` is required when:
- Command processes one argument at a time (`tail -f`, `gh pr merge`)
- Each line needs separate output (`echo`)

`-L1` is NOT required when:
- Command handles multiple arguments (`rm`, `sed`, `dirname`, `chmod`)

Note: `-I{}` implies `-L1` behavior, so explicit `-L1` is optional with `-I{}`.

## sort | head + pipefail

`sort | head -n N` causes SIGPIPE on `sort` (exit 141) with `pipefail` because `head` closes stdin after N lines while `sort` is still writing.
Use `awk` instead:

| sort \| head | sort \| awk equivalent |
|------|----------------|
| `sort \| head -n N` | `sort \| awk 'NR<=N'` |

## sort + locale

`sort` collates by `LC_ALL`, so the same input orders differently between shells: `en_US.UTF-8` ignores `/` and `.` at the primary weight and compares the letters that follow, while `C` compares bytes (`.` = 0x2E before `/` = 0x2F).
Prefix `LC_ALL=C` only where the keys can collate differently and something else consumes the order, so the rule does not fire on orderings no locale can change.

| Compared keys | Pattern | Examples |
|---------------|---------|----------|
| Carry punctuation, letters or mixed case, and something else consumes the order | `LC_ALL=C sort` | e.g. `bin/sync-agent-files.sh`, `files/home/kai/.asdf/plugins/hippocampus/bin/list-all` |
| No collation-sensitive consumer of the order — the differing segment is digits only, compared by `-V` rather than by collation, or displayed rather than consumed | bare `sort` | e.g. `files/usr/local/bin/backup.sh`, `files/usr/local/bin/shutdown.sh`, `.github/scripts/cleanup.sh` |

## grep + pipefail

`grep` returns exit code 1 when no match found, which fails the pipeline with `pipefail`.
Use `awk` instead:

| grep | awk equivalent |
|------|----------------|
| `grep pattern` | `awk '/pattern/'` |
| `grep -v pattern` | `awk '!/pattern/'` |
| `grep -E 'a\|b'` | `awk '/a\|b/'` |

## jq + pipefail

`jq -r` outputs `null` as string "null" and exits 0.
Use `jq -re` to fail on null/false:

| Scenario | Flag | Behavior |
|----------|------|----------|
| Null acceptable | `-r` | Outputs "null", exit 0 |
| Null is error | `-re` | Exits 1 on null/false |

```bash
# Fails pipeline if .value is null
value=$(echo "$json" | jq -re '.value')
```

## curl + retry

Add `--retry 5 --retry-all-errors` to curl calls hitting network services so a refused connection or an error status does not fail on the first attempt.
Place the retry flags immediately after the short-flag cluster.
Never add `--retry-delay` — it disables curl's exponential backoff, which is what makes the first retry fast and the last one patient.
Lower `--retry` rather than adding `--retry-max-time` wherever that backoff must fit a deadline the caller does not own, such as a job `timeout-minutes` — the sleeps are 1, 2, 4, 8 … seconds, so `--retry N` spends `2^N - 1` of the budget.
`--retry-max-time` gates only whether another retry is authorised, never an attempt already in flight, so with `--retry 5` any value above 15 changes nothing at all.

| curl target | Retry | Reason |
|-------------|-------|--------|
| API/status response (piped to `jq`/`awk`, or written with `-o`) | Yes | Failure happens before the success body streams, so a retry replays cleanly |
| Download streamed into an extractor or interpreter (e.g. `\| tar`, `\| sh`) | No | A mid-stream failure already wrote partial bytes to an unrewindable pipe; a retry re-sends from the start and corrupts the stream |

Bare `--retry` already covers timeouts, DNS resolution failures and curl's own HTTP set (408, 429, 500, 502, 503, 504, 522, 524), so `--retry-all-errors` is what buys a refused connection and, alongside `-f`, the remaining status codes.
Omit `--retry-all-errors` (keep `--retry`) when a 4xx is an expected outcome (e.g. probing an endpoint that returns 404), so the deterministic error is not retried.
Such a probe must drop `-f` as well, since `--fail` emits no body and exits non-zero: the body is what the probe reads, and the exit status aborts the script under `set -e` before the branch handling that outcome runs.

When the caller already retries, the table below overrides the one above.

| Caller shape | Retry | Reason |
|--------------|-------|--------|
| A poll loop whose only job is to wait for readiness (`while ! curl ...; do sleep 1; done`) | No | The loop's own `sleep` is the backoff, and curl's would compound with it |
| A loop that redoes work before the call (`until ensure_webhook; do ... done`) | Yes | The retry avoids repeating that work on a transient failure |

## kubectl wait

`--timeout` bounds a single attempt, so a wait that must eventually succeed needs its timeout derived from a real budget — a retry loop where blocking is safe, the time left where it is not.
Cold-boot ordering is unbounded, so any single-attempt timeout is a guess that eventually loses.

| Wait is | Pattern |
|---------|---------|
| A bootstrap precondition that must eventually succeed, with no external deadline | `until kubectl ... wait ... --timeout=10m; do sleep 1; done` |
| A bootstrap precondition under a systemd `TimeoutStartSec` | Derive `--timeout` from the budget left, and gate the dependent step on the wait succeeding |
| A test assertion with a real deadline (e.g. e2e pod completion) | bare `kubectl wait --timeout=...` |

Never `|| true` a wait whose result gates a later step — the suppressed failure turns the gate into a no-op while the gated step still runs.

A `kubectl get` existence guard around a repair step silently skips the whole block when the resource has not been created yet, and the script still exits successfully.

| Resource absence means | Pattern |
|------------------------|---------|
| The work is genuinely unnecessary (nothing to delete or restart) | `if [ -n "$(kubectl get ...)" ]; then` guard |
| Not created yet (applied asynchronously during cold boot) | `until [ "$(date +%s)" -ge "$deadline" ] \|\| kubectl get ...; do sleep 5; done` |

## Reference

If implementing CLI argument parsing:
  Read: `.claude/reference/bash/argument-parsing.md`

If implementing parallel execution:
  Read: `.claude/reference/bash/parallel-execution.md`
