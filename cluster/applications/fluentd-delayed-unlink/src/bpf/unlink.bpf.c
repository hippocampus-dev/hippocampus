#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>

#if defined(__TARGET_ARCH_x86)
#define SYSCALL_WRAPPER 1
#define SYS_PREFIX "__x64_"
#elif defined(__TARGET_ARCH_s390)
#define SYSCALL_WRAPPER 1
#define SYS_PREFIX "__s390x_"
#elif defined(__TARGET_ARCH_arm64)
#define SYSCALL_WRAPPER 1
#define SYS_PREFIX "__arm64_"
#else
#define SYSCALL_WRAPPER 0
#define SYS_PREFIX "__se_"
#endif

#define MAX_ENTRIES 10240
#define DIRECTORY_MAX 128
// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/limits.h#L13
#define PATH_MAX 4096

struct arg {
    const char *pathname;
};

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);
    __type(value, struct arg);
} args;

const volatile struct {
    gid_t this;
    u8 directory[DIRECTORY_MAX];
} tool_config;

struct event {
    gid_t tgid;
    pid_t pid;
    uid_t uid;
    u8 pathname[PATH_MAX];
} event;

// BPF programs will operate on 512-byte stack size
SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, struct event);
} unlink_heap;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, struct event);
} unlinkat_heap;

SEC(".maps") struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(u32));
	__uint(value_size, sizeof(u32));
} events;

static __always_inline bool filter_directory(u8 directory[DIRECTORY_MAX], char pathname[PATH_MAX]) {
    for (int i = 0; i < DIRECTORY_MAX; i++) {
        if (directory[i] == '\0') {
            break;
        }

        if (directory[i] != pathname[i]) {
            return false;
        }
    }
    return true;
}

// /sys/kernel/debug/tracing/events/syscalls/sys_enter_unlink/format
SEC("tracepoint/syscalls/sys_enter_unlink") int sys_enter_unlink(struct trace_event_raw_sys_enter *ctx) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.this == tgid || tool_config.this == ppid) {
        return 0;
    }

    const char *pathname = (const char *)ctx->args[0];

    // Pass an argument to the kprobe from userspace
    struct arg arg = {
        .pathname = pathname,
    };

    bpf_map_update_elem(&args, &pid, &arg, BPF_ANY);
    return 0;
}

SEC("kprobe/" SYS_PREFIX "sys_unlink") int BPF_KPROBE(sys_unlink) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&args, &pid);
    if (!argp) {
        return 0;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&unlink_heap, &zero);
    if (!eventp) {
        goto end;
    }

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->pathname[0] = '\0';
    if (bpf_probe_read_user(eventp->pathname, sizeof(eventp->pathname), argp->pathname) < 0) {
        goto end;
    }

    u8 directory[DIRECTORY_MAX];
    if (bpf_probe_read_kernel(directory, sizeof(directory), tool_config.directory) < 0) {
        goto end;
    }

    if (!filter_directory(directory, eventp->pathname)) {
        goto end;
    }

    bpf_override_return(ctx, 0);

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));
end:
    bpf_map_delete_elem(&args, &pid);
    return 0;
}

// /sys/kernel/debug/tracing/events/syscalls/sys_enter_unlinkat/format
SEC("tracepoint/syscalls/sys_enter_unlinkat") int sys_enter_unlinkat(struct trace_event_raw_sys_enter *ctx) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.this == tgid || tool_config.this == ppid) {
        return 0;
    }

    const char *pathname = (const char *)ctx->args[1];
    int flag = (int)ctx->args[2];

    // https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fcntl.h#L104
    if (flag & 0x200) {
        return 0;
    }

    // Pass an argument to the kprobe from userspace
    struct arg arg = {
        .pathname = pathname,
    };

    bpf_map_update_elem(&args, &pid, &arg, BPF_ANY);
    return 0;
}

SEC("kprobe/" SYS_PREFIX "sys_unlinkat") int BPF_KPROBE(sys_unlinkat) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&args, &pid);
    if (!argp) {
        return 0;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&unlinkat_heap, &zero);
    if (!eventp) {
        goto end;
    }

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->pathname[0] = '\0';
    if (bpf_probe_read_user(eventp->pathname, sizeof(eventp->pathname), argp->pathname) < 0) {
        goto end;
    }

    u8 directory[DIRECTORY_MAX];
    if (bpf_probe_read_kernel(directory, sizeof(directory), tool_config.directory) < 0) {
        goto end;
    }

    if (!filter_directory(directory, eventp->pathname)) {
        goto end;
    }

    bpf_override_return(ctx, 0);

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));
end:
    bpf_map_delete_elem(&args, &pid);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
