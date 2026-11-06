# BPF Probe Points That Can Fail an Operation

Which hook a `*.bpf.c` program has to attach to when observing an operation is not enough and it must refuse one, and what that choice costs.
`.claude/rules/bpf.md` carries the choice for a program that only observes.
The kernel behaviour below is checked against the tree `kernel-lab/fuse-bpf/experiment.sh` builds, under `kernel-lab/.build/fuse-bpf/linux/`.

Example: `cluster/applications/fluentd-delayed-unlink/src/bpf/unlink.bpf.c`

## Only a Syscall Kprobe or an lsm Hook Can Refuse

| Hook | Can refuse the operation |
|------|--------------------------|
| `fentry`/`fexit`/`kprobe` on a kernel function | No |
| `tracepoint/syscalls/sys_enter_*` | No |
| `uprobe`/`uretprobe` | No |
| `kprobe` on `SYS_PREFIX "sys_*"` | Yes, through `bpf_override_return` |
| `lsm/*` | Yes, through a negative return |

## bpf_override_return Needs an Error-Injectable Target

`bpf_override_return` needs its target to carry `ALLOW_ERROR_INJECTION`, which `arch/x86/include/asm/syscall_wrapper.h` and `arch/arm64/include/asm/syscall_wrapper.h` attach to every syscall wrapper from their `__SYS_STUB0` and `__SYS_STUBx` macros.
Every other function has to name itself, and no `vfs_*` function or `do_sys_*` entry point does - grep the tree for `ALLOW_ERROR_INJECTION(` to read the whole list, which under `fs/` covers btrfs internals alone.
So `vfs_unlink`, otherwise the architecture-agnostic place to watch a deletion, can only observe one.
The target is not the only thing checked: `kernel/events/core.c` rejects a program carrying `kprobe_override` that is attached to anything but a kprobe, so a `uprobe` cannot reach the helper whatever it targets.

## The Wrapper Is Passed pt_regs, So Its Arguments Come From a Tracepoint

Those same macros give the wrapper the signature `long __x64_sys_unlink(const struct pt_regs *regs)`, so a `BPF_KPROBE` on it receives the register block where the syscall's first argument would be.
Capture the arguments in a `tracepoint/syscalls/sys_enter_*` keyed on the pid, and look them up again in the kprobe that calls `bpf_override_return`.
`cluster/applications/fluentd-delayed-unlink/src/bpf/unlink.bpf.c` pairs the two this way for both `unlink` and `unlinkat`, and its `#if defined(__TARGET_ARCH_x86)` cascade defining `SYS_PREFIX` is the one to copy.

## An lsm Program Refuses Without a Syscall Hook

`security/bpf/hooks.c` registers `bpf_lsm_<hook>` as an ordinary LSM hook and the callers hand its result straight back, `security_file_open()` being `call_int_hook(file_open, 0, file)` followed by a return of anything non-zero.
An `lsm/*` program therefore refuses by returning a negative errno, with no per-architecture spelling and no `ALLOW_ERROR_INJECTION` involved, but only at the hooks `include/linux/lsm_hook_defs.h` declares.
`insight/src/vfs/vfs.bpf.c` attaches `lsm/file_open` for observation alone, so nothing in the tree exercises the refusal yet.
