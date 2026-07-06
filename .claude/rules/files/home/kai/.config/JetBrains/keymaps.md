---
paths:
  - "files/home/kai/.config/JetBrains/**/keymaps/*.xml"
---

* Read `files/home/kai/.config/JetBrains/IntelliJIdea2024.3/keymaps/Mine.xml` to identify used keys before suggesting new shortcuts
* Extract `keyboard-shortcut` elements to find used key combinations
* Use the modifier pattern table below to select appropriate key combination

## Modifier Patterns

| Pattern | Purpose | Examples |
|---------|---------|----------|
| `alt + letter` | Frequent single operations | `alt j` (GotoFile), `alt s` (GotoSymbol), `alt t` (Terminal) |
| `ctrl + letter` | Paired operations (Open↔Close) | `ctrl t` (Terminal.CloseTab pairs with `alt t`) |
| `ctrl + symbol` | Editor actions (comment, zoom) | `ctrl slash` (Comment), `ctrl equals/minus` (FontSize) |
| `ctrl alt + number` | Tool windows | `ctrl alt 1` (Run), `ctrl alt 2` (Debug), `ctrl alt 3` (Build) |
| `ctrl alt + letter` | Extended/compound operations | `ctrl alt f` (FindInPath), `ctrl alt g` (ReplaceInPath) |
| `ctrl alt + arrow` | Tab/window navigation | `ctrl alt left/right` (PreviousTab/NextTab) |
| `shift ctrl + letter` | VCS and special operations | `shift ctrl c` (Commit), `shift ctrl p` (Push), `shift ctrl d` (Diff) |
| `alt + arrow` | In-file navigation (prev/next) | `alt left/right` (Diff.PrevChange/NextChange) |
| `ctrl + arrow` | Editor/window resize | `ctrl up/down` (ResizeToolWindow) |
| `ctrl + n/p` | Emacs-style navigation | `ctrl n` (Down), `ctrl p` (Up) |

## Selection Guidelines

| Category | Recommended Pattern | Rationale |
|----------|---------------------|-----------|
| Navigation/Goto | `alt + letter` | Single keystroke for frequent use |
| Close/Toggle (paired) | `ctrl + letter` | Pairs with `alt + letter` for same feature |
| Search/Find extended | `ctrl alt + letter` | Pair with base action (e.g., Find → FindInPath) |
| Tab/Window navigation | `ctrl alt + arrow` | Distinct from in-file navigation |
| VCS operations | `shift ctrl + letter` | Consistent with existing VCS shortcuts |
| Tool window focus | `ctrl alt + number` | Numeric ordering by importance |
| Recursive/extended | Add `ctrl` to base | e.g., `alt h` → `ctrl alt h` for recursive |

## Avoid

| Pattern | Reason |
|---------|--------|
| `shift ctrl alt + letter` | Conflicts with system shortcuts (e.g., `shift ctrl alt c`, `shift ctrl alt s`) |
