#include "../helpers.h"
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>

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
} read_args;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);
    __type(value, struct arg);
} write_args;

const volatile struct {
    gid_t tgid;
} tool_config;

enum kind { request, response };

struct event {
    gid_t tgid;
    pid_t pid;
    uid_t uid;
    u32 len;
    u8 buf[MAX_DATA_SIZE];
    enum kind kind;
} event;

// BPF programs will operate on 512-byte stack size
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

SEC("uprobe/SSL_read") int BPF_KPROBE(enter_ssl_read, void *ssl, void *buf, int num) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tool_config.tgid != tgid && tool_config.tgid != ppid) {
        return 0;
    }

    struct arg arg = {
        .buf = buf,
    };

    bpf_map_update_elem(&read_args, &pid, &arg, BPF_ANY);
    return 0;
}

SEC("uretprobe/SSL_read") int BPF_KRETPROBE(exit_ssl_read, int ret) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&read_args, &pid);
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
    eventp->len = PT_REGS_RC(ctx);
    eventp->kind = response;
    u32 buf_copy_size = min((size_t)MAX_DATA_SIZE, (size_t)eventp->len);
    bpf_probe_read_user(eventp->buf, buf_copy_size, (u8 *)argp->buf);

    if (eventp->len != -1) {
        bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));
    }
end:
    bpf_map_delete_elem(&read_args, &pid);
    return 0;
}

SEC("uprobe/SSL_write") int BPF_KPROBE(enter_ssl_write, void *ssl, void *buf, int num) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tool_config.tgid != tgid && tool_config.tgid != ppid) {
        return 0;
    }

    struct arg arg = {
        .buf = buf,
    };

    bpf_map_update_elem(&write_args, &pid, &arg, BPF_ANY);
    return 0;
}

SEC("uretprobe/SSL_write") int BPF_KRETPROBE(exit_ssl_write, int ret) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&write_args, &pid);
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
    eventp->len = PT_REGS_RC(ctx);
    eventp->kind = request;
    u32 buf_copy_size = min((size_t)MAX_DATA_SIZE, (size_t)eventp->len);
    bpf_probe_read_user(eventp->buf, buf_copy_size, (u8 *)argp->buf);
    if (eventp->len != -1) {
        bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));
    }

end:
    bpf_map_delete_elem(&write_args, &pid);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
