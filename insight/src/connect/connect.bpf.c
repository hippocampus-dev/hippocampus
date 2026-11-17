#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_endian.h>

#if defined(__TARGET_ARCH_x86)
#define SYS_PREFIX "__x64_"
#elif defined(__TARGET_ARCH_s390)
#define SYS_PREFIX "__s390x_"
#elif defined(__TARGET_ARCH_arm64)
#define SYS_PREFIX "__arm64_"
#else
#define SYS_PREFIX "__se_"
#endif

#define MAX_ENTRIES 10240

const volatile struct {
    gid_t tgid;
} tool_config;

struct arg {
    int fd;
    u32 sin_addr;
    u32 sin_port;
    u32 closed;
} arg;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, int);
    __type(value, struct arg);
} fds;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, u32);
    __type(value, struct arg);
} args;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, struct event);
} heap;

struct event {
    gid_t tgid;
    pid_t pid;
    uid_t uid;
    int fd;
    u32 sin_addr;
    u32 sin_port;
    int ret;
} event;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u32));
} events;

// /sys/kernel/debug/tracing/events/syscalls/sys_enter_connect/format
SEC("tracepoint/syscalls/sys_enter_connect") int sys_enter_connect(struct trace_event_raw_sys_enter *ctx) {
    int fd = (int)ctx->args[0];
    struct sockaddr *uaddr = (struct sockaddr*)ctx->args[1];

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tgid != tool_config.tgid && ppid != tool_config.tgid) {
        return 0;
    }

    bpf_printk("connect: fd=%d\n", fd);

    u16 address_family = 0;
    bpf_probe_read_user(&address_family, sizeof(address_family), &uaddr->sa_family);
    if (address_family == 2) {
        struct sockaddr_in *uaddr_in = (struct sockaddr_in *)uaddr;

        u32 sin_addr = 0;
        bpf_probe_read_user(&sin_addr, sizeof(sin_addr), &uaddr_in->sin_addr.s_addr);

        u32 sin_port = 0;
        bpf_probe_read_user(&sin_port, sizeof(sin_port), &uaddr_in->sin_port);

        struct arg arg = {
                .fd = fd,
                .sin_addr = sin_addr,
                .sin_port = bpf_ntohs(sin_port),
                .closed = 0,
        };

        bpf_map_update_elem(&fds, &fd, &arg, BPF_ANY);
    }
    return 0;
}

SEC("kprobe/" SYS_PREFIX "sys_poll") int BPF_KPROBE(sys_poll, int _, struct pollfd *ufds) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tgid != tool_config.tgid && ppid != tool_config.tgid) {
        return 0;
    }

    int fd;
    bpf_probe_read_user(&fd, sizeof(fd), &ufds->fd);
    struct arg *argp = bpf_map_lookup_elem(&fds, &fd);
    if (!argp) {
        return 0;
    }
    if (argp->closed == 1) {
        bpf_override_return(ctx, -1);
    }
    bpf_map_update_elem(&args, &pid, argp, BPF_ANY);

    return 0;
}

SEC("kretprobe/" SYS_PREFIX "sys_poll") int BPF_KRETPROBE(sys_poll_ret, int ret) {
    if (ret == -1) {
        return 0;
    }

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&args, &pid);
    if (!argp) {
        return 0;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&heap, &zero);
    if (!eventp) {
        return 0;
    }

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->fd = (int)argp->fd;
    eventp->sin_addr = (u32)argp->sin_addr;
    eventp->sin_port = (u32)argp->sin_port;
    eventp->ret = ret;
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));

    bpf_map_delete_elem(&args, &pid);
    return 0;
}

SEC("kprobe/" SYS_PREFIX "sys_ppoll") int BPF_KPROBE(sys_ppoll, int _, struct pollfd *ufds) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tgid != tool_config.tgid && ppid != tool_config.tgid) {
        return 0;
    }

    int fd;
    bpf_probe_read_user(&fd, sizeof(fd), &ufds->fd);
    struct arg *argp = bpf_map_lookup_elem(&fds, &fd);
    if (!argp) {
        return 0;
    }
    if (argp->closed == 1) {
        bpf_override_return(ctx, -1);
    }
    bpf_map_update_elem(&args, &pid, argp, BPF_ANY);

    return 0;
}

SEC("kretprobe/" SYS_PREFIX "sys_ppoll") int BPF_KRETPROBE(sys_ppoll_ret, int ret) {
    if (ret == -1) {
        return 0;
    }

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&args, &pid);
    if (!argp) {
        return 0;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&heap, &zero);
    if (!eventp) {
        return 0;
    }

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->fd = (int)argp->fd;
    eventp->sin_addr = (u32)argp->sin_addr;
    eventp->sin_port = (u32)argp->sin_port;
    eventp->ret = ret;
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));

    bpf_map_delete_elem(&args, &pid);
    return 0;
}

// /sys/kernel/debug/tracing/events/syscalls/sys_enter_close/format
SEC("tracepoint/syscalls/sys_enter_close") int sys_enter_close(struct trace_event_raw_sys_enter *ctx) {
    int fd = (int)ctx->args[0];

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tgid != tool_config.tgid && ppid != tool_config.tgid) {
        goto end;
    }

end:
    bpf_map_delete_elem(&fds, &fd);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
