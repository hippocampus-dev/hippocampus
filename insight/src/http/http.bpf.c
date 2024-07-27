#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

#define MAX_ENTRIES 10240
#define MAX_DATA_SIZE 30720

struct arg {
    void *buf;
};

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);
    __type(value, struct arg);
} sendtoargs;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);
    __type(value, struct arg);
} recvfromargs;

const volatile struct {
    gid_t tgid;
} tool_config;

enum kind { request, response };

struct event {
    gid_t tgid;
    pid_t pid;
    uid_t uid;
    u8 buf[MAX_DATA_SIZE];
    enum kind kind;
} event;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, struct event);
} heap;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u32));
} events;

// /sys/kernel/debug/tracing/events/syscalls/sys_enter_sendto/format
SEC("tracepoint/syscalls/sys_enter_sendto") int sys_enter_sendto(struct trace_event_raw_sys_enter *ctx) {
    void *buf = (void *)ctx->args[1];

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tgid != tool_config.tgid && ppid != tool_config.tgid) {
        return 0;
    }

    struct arg arg = {
            .buf = buf,
    };

    bpf_map_update_elem(&sendtoargs, &pid, &arg, BPF_ANY);
    return 0;
}

// /sys/kernel/debug/tracing/events/syscalls/sys_exit_sendto/format
SEC("tracepoint/syscalls/sys_exit_sendto") int sys_exit_sendto(struct trace_event_raw_sys_exit *ctx) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&sendtoargs, &pid);
    if (!argp) {
        return 0;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&heap, &zero);
    if (!eventp) {
        goto end;
    }

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->kind = request;
    bpf_probe_read_user_str(eventp->buf, sizeof(eventp->buf), (u8 *)argp->buf);
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));

end:
    bpf_map_delete_elem(&sendtoargs, &pid);
    bpf_map_delete_elem(&heap, &zero);
    return 0;
}

// /sys/kernel/debug/tracing/events/syscalls/sys_enter_recvfrom/format
SEC("tracepoint/syscalls/sys_enter_recvfrom") int sys_enter_recvfrom(struct trace_event_raw_sys_enter *ctx) {
    void *buf = (void *)ctx->args[1];

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tgid != tool_config.tgid && ppid != tool_config.tgid) {
        return 0;
    }

    struct arg arg = {
        .buf = buf,
    };

    bpf_map_update_elem(&recvfromargs, &pid, &arg, BPF_ANY);
    return 0;
}

// /sys/kernel/debug/tracing/events/syscalls/sys_exit_recvfrom/format
SEC("tracepoint/syscalls/sys_exit_recvfrom") int sys_exit_recvfrom(struct trace_event_raw_sys_exit *ctx) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&recvfromargs, &pid);
    if (!argp) {
        return 0;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&heap, &zero);
    if (!eventp) {
        goto end;
    }

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->kind = response;
    bpf_probe_read_user_str(eventp->buf, sizeof(eventp->buf), (u8 *)argp->buf);
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));

end:
    bpf_map_delete_elem(&recvfromargs, &pid);
    bpf_map_delete_elem(&heap, &zero);
    return 0;
}
