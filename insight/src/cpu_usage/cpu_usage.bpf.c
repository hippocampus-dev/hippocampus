#include "../helpers.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>

#define MAX_ENTRIES 4194304
#define TASK_COMM_LEN 16
#define MAX_SLOTS 32

const volatile struct {
    gid_t tgid;
} tool_config;

struct event {
    u32 histogram[MAX_SLOTS];
    u8 comm[TASK_COMM_LEN];
};

struct event initial_event;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, u32);
    __type(value, struct event);
    __uint(map_flags, BPF_F_NO_PREALLOC);
} events;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, u32);
    __type(value, u64);
    __uint(map_flags, BPF_F_NO_PREALLOC);
} start;

SEC("kprobe/finish_task_switch.isra.0") int BPF_KPROBE(finish_task_switch, struct task_struct *prev) {
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    gid_t tgid = BPF_CORE_READ(task, tgid);
    pid_t pid = BPF_CORE_READ(task, pid);
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    gid_t prev_tgid = BPF_CORE_READ(prev, tgid);
    pid_t prev_pid = BPF_CORE_READ(prev, pid);
    u32 prev_ppid = BPF_CORE_READ(prev, real_parent, tgid);

    u64 ts = bpf_ktime_get_ns();

    if (tool_config.tgid && (tgid == tool_config.tgid || ppid == tool_config.tgid)) {
        bpf_map_update_elem(&start, &pid, &ts, 0);
    }
    if (tool_config.tgid && (prev_tgid == tool_config.tgid || prev_ppid == tool_config.tgid)) {
        u64 *tsp = bpf_map_lookup_elem(&start, &prev_pid);
        if (tsp != 0 && ts > *tsp) {
            struct event *eventp;
            eventp = bpf_map_lookup_elem(&events, &prev_tgid);
            if (!eventp) {
                bpf_map_update_elem(&events, &prev_tgid, &initial_event, 0);
                eventp = bpf_map_lookup_elem(&events, &prev_tgid);
                if (!eventp) {
                    return 0;
                }
                bpf_probe_read_kernel_str(&eventp->comm, sizeof(prev->comm), prev->comm);
            }

            u64 delta = ts - *tsp;
            delta /= 1000000;
            u64 slot = log2l(delta);
            if (slot >= MAX_SLOTS) {
                slot = MAX_SLOTS - 1;
            }
            __sync_fetch_and_add(&eventp->histogram[slot], 1);
        }
    }
    return 0;
}
