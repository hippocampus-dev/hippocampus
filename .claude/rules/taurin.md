---
paths:
  - "taurin/**"
---

* Use `LogicalSize` (not `PhysicalSize`) when calling `window.setSize()` with DOM measurements (`offsetWidth`, `offsetHeight`)
* Set `minWidth` and `minHeight` in `tauri.conf.json` as safety net for dynamic window sizing

## Window Sizing

| Size Type | Input Unit | Use Case |
|-----------|------------|----------|
| `LogicalSize` | CSS pixels | DOM measurements (`offsetWidth`, `offsetHeight`) |
| `PhysicalSize` | Device pixels | Screen/display APIs |

DOM APIs return CSS pixels (logical), which match `LogicalSize`. Using `PhysicalSize` with DOM measurements causes incorrect sizing on high-DPI displays.
