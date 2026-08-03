# tmux Targeting Pattern

How to target a specific pane from code that runs inside a tmux session.

## Why It Matters

`display-message`, `select-pane`, `setw`, `send-keys`, `split-window` and `resize-pane` resolve to the focused pane when `-t` is omitted.
With several sessions sharing one tmux server that silently hits another session's pane.
An empty `-t ""` resolves the same way, so an unset variable is not a safe target either.

## Choosing a Target

| Caller | Target |
|--------|--------|
| Code running inside a session | `-t "$TMUX_PANE"` (`files/home/kai/bin/claudex` propagates it into the sandbox with `-E TMUX_PANE`) |
| Launcher building a layout with `split-window -P -F "#{pane_id}"` | `-t <captured pane id>` |
| Launcher splitting repeatedly | `-t <pane captured by the previous iteration>`, since `-t "$TMUX_PANE"` would land every split on the launcher pane and flatten the layout |
| Layout script that captures no pane id | Exempt, because following the focus is its mechanism |

Read the current pane id from `$TMUX_PANE` rather than asking tmux for it, and guard on the pane-id variable itself rather than on `$TMUX`.

## Probing a Pane

| Subcommand | Exit code for a missing pane |
|------------|------------------------------|
| `display-message` | 0 |
| `list-panes` | 1 |
| `show-window-options` | 1 |
| `send-keys` | 1 |
| `setw` | 1 |
| `kill-pane` | 1 |

`display-message` cannot serve as a probe because it exits 0 either way.

```bash
if [ -z "$TMUX_PANE" ]; then
  exit 1
fi

tmux list-panes -t "$TMUX_PANE" > /dev/null
```

A launcher that does irreversible work before its first tmux call needs this probe after the guard, so it fails before that work rather than after.

## Failing Inside an EXIT Trap

`set -e` aborts the whole trap when any tmux call fails, skipping everything after it.
Suppress the failure or gate the call on a liveness check.

```bash
cleanup() {
  tmux setw -t "$TMUX_PANE" monitor-silence 0 2>/dev/null || true

  if tmux list-panes -t "$pane" > /dev/null 2>&1; then
    tmux send-keys -t "$pane" C-c
  fi
  tmux kill-pane -t "$pane" 2>/dev/null || true
}
```

## Window Options

Window options such as `monitor-silence` apply to the whole window, so panes split into one window share them.
`-t` selects which window the option lands on, not how far it reaches.

## Reaching a Pane's Own Work

`#{pane_pid}` is the pane's shell, not the command it ran, so a name that command derived from its own `$$` is unreachable from the caller, and the pane may already be gone by the time cleanup runs.
A caller that has to clean up something the pane's command creates must pass the name in rather than reconstruct it afterwards.
The pane also inherits the cwd of the process that ran `split-window`, so build any path from the same base you passed down.
