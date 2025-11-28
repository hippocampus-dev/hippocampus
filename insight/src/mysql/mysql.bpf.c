#include "../helpers.h"
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>

#define MAX_ENTRIES 10240
#define MAX_DATA_SIZE 30720
#define MYSQL_DEFAULT_PORT 3306
#define AF_INET 2

enum mysql_conn_state {
    CONN_STATE_CONNECTING = 0,      // Just connected, waiting for server greeting
    CONN_STATE_SERVER_GREETING = 1,  // Received server greeting (Initial Handshake)
    CONN_STATE_CLIENT_AUTH = 2,      // Sent client authentication
    CONN_STATE_ESTABLISHED = 3,      // Handshake complete, normal MySQL traffic
    CONN_STATE_INVALID = 4,          // Not a MySQL connection
};

struct connection_info {
    int fd;
    u8 sequence_id;
    u8 is_mysql;
    u16 port;
    enum mysql_conn_state state;
    u8 last_sequence_id;
};

struct arg {
    int fd;
    void *buf;
    size_t len;
};

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, int);
    __type(value, struct connection_info);
} connections;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);
    __type(value, struct arg);
} send_args;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);
    __type(value, struct arg);
} recv_args;

const volatile struct {
    gid_t tgid;
    u16 target_port;
} tool_config = {
    .target_port = MYSQL_DEFAULT_PORT,
};

enum packet_direction {
    CLIENT_TO_SERVER,
    SERVER_TO_CLIENT
};

struct event {
    gid_t tgid;
    pid_t pid;
    uid_t uid;
    int fd;
    u32 len;
    u8 sequence_id;
    u8 buf[MAX_DATA_SIZE];
    enum packet_direction direction;
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

static __always_inline int is_mysql_handshake_packet(u8 *buf, size_t len, u8 sequence_id, int is_server) {
    if (len < 5) {
        return 0;
    }

    if (is_server && sequence_id == 0) {
        return buf[4] == 0x0a;
    } else if (!is_server && sequence_id == 1) {
        return len > 32;
    } else if (is_server && sequence_id >= 2) {
        return buf[4] == 0x00 || buf[4] == 0xFF || buf[4] == 0xFE;
    } else if (!is_server && sequence_id >= 3) {
        return 1;
    }

    return 0;
}

static __always_inline void update_connection_state(struct connection_info *conn, u8 *buf, size_t len, u8 sequence_id, int is_server) {
    switch (conn->state) {
        case CONN_STATE_CONNECTING:
            if (is_server && sequence_id == 0 && len > 10 && buf[4] == 0x0a) {
                conn->state = CONN_STATE_SERVER_GREETING;
                conn->last_sequence_id = sequence_id;
            }
            break;

        case CONN_STATE_SERVER_GREETING:
            if (!is_server && sequence_id == 1) {
                conn->state = CONN_STATE_CLIENT_AUTH;
                conn->last_sequence_id = sequence_id;
            }
            break;

        case CONN_STATE_CLIENT_AUTH:
            if (is_server) {
                if (len > 4) {
                    if (buf[4] == 0x00) {
                        conn->state = CONN_STATE_ESTABLISHED;
                        conn->is_mysql = 1;
                        conn->last_sequence_id = sequence_id;
                    } else if (buf[4] == 0xFF) {
                        conn->state = CONN_STATE_INVALID;
                        conn->is_mysql = 0;
                    }
                }
            }
            break;

        case CONN_STATE_ESTABLISHED:
            conn->last_sequence_id = sequence_id;
            break;

        case CONN_STATE_INVALID:
            break;
    }
}

static __always_inline int is_mysql_packet(u8 *buf, size_t len) {
    if (len < 4) {
        return 0;
    }

    u32 packet_length = buf[0] | (buf[1] << 8) | (buf[2] << 16);

    if (packet_length == 0 || packet_length > MAX_DATA_SIZE) {
        return 0;
    }

    return 1;
}

// Track MySQL connections via connect() syscall
SEC("tracepoint/syscalls/sys_enter_connect")
int sys_enter_connect(struct trace_event_raw_sys_enter *ctx) {
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

    u16 address_family = 0;
    bpf_probe_read_user(&address_family, sizeof(address_family), &uaddr->sa_family);

    if (address_family == AF_INET) {
        struct sockaddr_in *uaddr_in = (struct sockaddr_in *)uaddr;

        u16 sin_port = 0;
        bpf_probe_read_user(&sin_port, sizeof(sin_port), &uaddr_in->sin_port);
        sin_port = bpf_ntohs(sin_port);

        // Only track connections to MySQL port (default 3306)
        if (sin_port == 3306 || (tool_config.target_port != 0 && sin_port == tool_config.target_port)) {
            struct connection_info conn = {
                .fd = fd,
                .sequence_id = 0,
                .is_mysql = 0,  // Not confirmed as MySQL until handshake
                .port = sin_port,
                .state = CONN_STATE_CONNECTING,
                .last_sequence_id = 0,
            };

            bpf_map_update_elem(&connections, &fd, &conn, BPF_ANY);
        }
    }

    return 0;
}

// Note: sendto is used for both UDP and TCP sockets
SEC("tracepoint/syscalls/sys_enter_sendto")
int sys_enter_sendto(struct trace_event_raw_sys_enter *ctx) {
    int fd = (int)ctx->args[0];
    void *buf = (void *)ctx->args[1];
    size_t len = (size_t)ctx->args[2];

    struct connection_info *conn = bpf_map_lookup_elem(&connections, &fd);
    if (!conn) {
        return 0;
    }

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    pid_t pid = __pid_tgid;

    struct arg arg = {
        .fd = fd,
        .buf = buf,
        .len = len,
    };

    bpf_map_update_elem(&send_args, &pid, &arg, BPF_ANY);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_sendto")
int sys_exit_sendto(struct trace_event_raw_sys_exit *ctx) {
    ssize_t ret = ctx->ret;
    if (ret <= 0) {
        return 0;
    }

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&send_args, &pid);
    if (!argp) {
        return 0;
    }

    struct connection_info *conn = bpf_map_lookup_elem(&connections, &argp->fd);
    if (!conn) {
        goto cleanup;
    }

    // Skip if connection is already marked as invalid
    if (conn->state == CONN_STATE_INVALID) {
        goto cleanup;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&heap, &zero);
    if (!eventp) {
        goto cleanup;
    }

    // Read packet data
    u32 read_size = min((size_t)MAX_DATA_SIZE, argp->len);
    bpf_probe_read_user(eventp->buf, read_size, argp->buf);

    // Extract sequence ID from packet header
    u8 sequence_id = eventp->buf[3];

    // Update handshake state for CLIENT_TO_SERVER packets
    update_connection_state(conn, eventp->buf, read_size, sequence_id, 0);

    // Only skip packets from connections we've confirmed are NOT MySQL
    if (conn->state == CONN_STATE_INVALID) {
        goto cleanup;
    }

    // Basic MySQL packet validation
    if (!is_mysql_packet(eventp->buf, read_size)) {
        // If we haven't seen a proper handshake yet, mark as invalid
        if (conn->state == CONN_STATE_CONNECTING) {
            conn->state = CONN_STATE_INVALID;
        }
        goto cleanup;
    }


    // Update connection sequence tracking
    conn->sequence_id = sequence_id;

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->fd = argp->fd;
    eventp->len = read_size;  // Use actual read size, not requested size
    eventp->sequence_id = sequence_id;
    eventp->direction = CLIENT_TO_SERVER;


    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));

cleanup:
    bpf_map_delete_elem(&send_args, &pid);
    bpf_map_delete_elem(&heap, &zero);
    return 0;
}

// Note: recvfrom is used for both UDP and TCP sockets
SEC("tracepoint/syscalls/sys_enter_recvfrom")
int sys_enter_recvfrom(struct trace_event_raw_sys_enter *ctx) {
    int fd = (int)ctx->args[0];
    void *buf = (void *)ctx->args[1];
    size_t len = (size_t)ctx->args[2];

    struct connection_info *conn = bpf_map_lookup_elem(&connections, &fd);
    if (!conn) {
        return 0;
    }

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    pid_t pid = __pid_tgid;

    struct arg arg = {
        .fd = fd,
        .buf = buf,
        .len = len,
    };

    bpf_map_update_elem(&recv_args, &pid, &arg, BPF_ANY);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_recvfrom")
int sys_exit_recvfrom(struct trace_event_raw_sys_exit *ctx) {
    ssize_t ret = ctx->ret;
    if (ret <= 0) {
        return 0;
    }

    u64 __pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = __pid_tgid >> 32;
    pid_t pid = __pid_tgid;

    struct arg *argp = bpf_map_lookup_elem(&recv_args, &pid);
    if (!argp) {
        return 0;
    }

    struct connection_info *conn = bpf_map_lookup_elem(&connections, &argp->fd);
    if (!conn) {
        goto cleanup;
    }

    // Skip if connection is already marked as invalid
    if (conn->state == CONN_STATE_INVALID) {
        goto cleanup;
    }

    int zero = 0;
    struct event *eventp = bpf_map_lookup_elem(&heap, &zero);
    if (!eventp) {
        goto cleanup;
    }

    // Read packet data
    u32 read_size = min((size_t)MAX_DATA_SIZE, (size_t)ret);
    bpf_probe_read_user(eventp->buf, read_size, argp->buf);

    // Extract sequence ID from packet header
    u8 sequence_id = eventp->buf[3];

    // Update handshake state for SERVER_TO_CLIENT packets
    update_connection_state(conn, eventp->buf, read_size, sequence_id, 1);

    // Only skip packets from connections we've confirmed are NOT MySQL
    if (conn->state == CONN_STATE_INVALID) {
        goto cleanup;
    }

    // Basic MySQL packet validation
    if (!is_mysql_packet(eventp->buf, read_size)) {
        // If we haven't seen a proper handshake yet, mark as invalid
        if (conn->state == CONN_STATE_CONNECTING) {
            conn->state = CONN_STATE_INVALID;
        }
        goto cleanup;
    }


    // Update connection sequence tracking
    conn->sequence_id = sequence_id;

    eventp->tgid = tgid;
    eventp->pid = pid;
    eventp->uid = bpf_get_current_uid_gid();
    eventp->fd = argp->fd;
    eventp->len = read_size;  // Use actual read size
    eventp->sequence_id = sequence_id;
    eventp->direction = SERVER_TO_CLIENT;


    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, eventp, sizeof(*eventp));

cleanup:
    bpf_map_delete_elem(&recv_args, &pid);
    bpf_map_delete_elem(&heap, &zero);
    return 0;
}



// Clean up on close()
SEC("tracepoint/syscalls/sys_enter_close")
int sys_enter_close(struct trace_event_raw_sys_enter *ctx) {
    int fd = (int)ctx->args[0];

    struct connection_info *conn = bpf_map_lookup_elem(&connections, &fd);
    if (conn) {
        bpf_map_delete_elem(&connections, &fd);
    }

    return 0;
}

char _license[] SEC("license") = "GPL";
