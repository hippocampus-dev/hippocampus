#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>

#define MAX_ENTRIES 8192
#define ADDR_LEN 16
#define TASK_COMM_LEN 16

const volatile struct {
    u32 daddr_v4[ADDR_LEN];
    u32 daddr_v4_len;
    u8 daddr_v6[ADDR_LEN][16];
    u32 daddr_v6_len;
} tool_config;

enum protocol { ipv4, ipv6 };

struct event {
    gid_t tgid;
    pid_t pid;
    uid_t uid;
    u32 daddr_v4;
    u8 daddr_v6[16];
	u8 comm[TASK_COMM_LEN];
    enum protocol protocol;
} event;

SEC(".maps") struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, MAX_ENTRIES);
	__type(key, u32);
	__type(value, struct sock*);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} sockets;

SEC(".maps") struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(u32));
	__uint(value_size, sizeof(u32));
} events;

SEC("kprobe/tcp_v4_connect") int BPF_KPROBE(tcp_v4_connect, struct sock *sk, struct sockaddr *uaddr, int addr_len) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    bpf_map_update_elem(&sockets, &pid, &sk, 0);
    return 0;
}

static __always_inline bool filter_daddr_v4(u32 daddr) {
	if (tool_config.daddr_v4_len == 0) {
        return true;
    }

	for (int i = 0; i < tool_config.daddr_v4_len; i++) {
		if (daddr == tool_config.daddr_v4[i]) {
            return true;
        }
	}
	return false;
}

SEC("kretprobe/tcp_v4_connect") int BPF_KRETPROBE(tcp_v4_connect_ret, int ret) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct sock **skpp = bpf_map_lookup_elem(&sockets, &pid);
    if (!skpp) {
        return 0;
    }

    if (ret) {
        goto end;
    }

    struct sock *sk = *skpp;

    u32 daddr_v4 = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    if (!filter_daddr_v4(daddr_v4)) {
        goto end;
    }

    uid_t uid = bpf_get_current_uid_gid();
    struct event event = {
            .tgid = tgid,
            .pid = pid,
            .uid = uid,
            .protocol = ipv4,
    };

    BPF_CORE_READ_INTO(&event.daddr_v4, sk, __sk_common.skc_daddr);

    bpf_get_current_comm(event.comm, sizeof(event.comm));
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
end:
    bpf_map_delete_elem(&sockets, &pid);
    return 0;
}

static __always_inline bool filter_daddr_v6(u8 daddr[16]) {
    if (tool_config.daddr_v6_len == 0) {
        return true;
    }

    for (int i = 0; i < tool_config.daddr_v6_len; i++) {
        bool m = true;
        for (int j = 0; j < 16; j++) {
            if (daddr[j] != tool_config.daddr_v6[i][j]) {
                m = false;
            }
        }
        if (m) {
            return true;
        }
    }
    return false;
}

SEC("kprobe/tcp_v6_connect") int BPF_KPROBE(tcp_v6_connect, struct sock *sk, struct sockaddr *uaddr, int addr_len) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    bpf_map_update_elem(&sockets, &pid, &sk, 0);
    return 0;
}

SEC("kretprobe/tcp_v6_connect") int BPF_KRETPROBE(tcp_v6_connect_ret, int ret) {
    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct sock **skpp = bpf_map_lookup_elem(&sockets, &pid);
    if (!skpp) {
        return 0;
    }

    if (ret) {
        goto end;
    }

    struct sock *sk = *skpp;

    u8 daddr_v6[16];
    BPF_CORE_READ_INTO(&daddr_v6, sk, __sk_common.skc_v6_daddr.in6_u.u6_addr32);
    if (!filter_daddr_v6(daddr_v6)) {
        goto end;
    }

    uid_t uid = bpf_get_current_uid_gid();
    struct event event = {
            .tgid = tgid,
            .pid = pid,
            .uid = uid,
            .protocol = ipv6,
    };

    BPF_CORE_READ_INTO(&event.daddr_v6, sk, __sk_common.skc_v6_daddr.in6_u.u6_addr32);

    bpf_get_current_comm(event.comm, sizeof(event.comm));
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
end:
    bpf_map_delete_elem(&sockets, &pid);
    return 0;
}
