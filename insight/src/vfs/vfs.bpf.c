#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#define FILE_OP_READ 0
#define FILE_OP_WRITE 1

#define FILENAME_LEN 256

#define S_IFMT 00170000
#define S_IFSOCK 0140000
#define S_IFLNK 0120000
#define S_IFREG 0100000
#define S_IFBLK 0060000
#define S_IFDIR 0040000
#define S_IFCHR 0020000
#define S_IFIFO 0010000
#define S_ISREG(m) (((m)&S_IFMT) == S_IFREG)

struct event {
    u32 pid;
    u32 tgid;
    u32 uid;
    u32 __pad;
    u64 ts;
    u64 size;
    s64 ret;
    u8 op;
    char comm[TASK_COMM_LEN];
    char filename[FILENAME_LEN];
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} events SEC(".maps");

struct path_entry {
    char filename[FILENAME_LEN];
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 8192);
    __type(key, u64);
    __type(value, struct path_entry);
} path_cache SEC(".maps");

const volatile struct {
    gid_t tgid;
} tool_config;

static __always_inline bool starts_with(const char *str, const char *prefix, int max_len) {
    for (int i = 0; i < max_len; i++) {
        if (prefix[i] == '\0') {
            return true;
        }
        if (str[i] != prefix[i]) {
            return false;
        }
    }
    return false;
}

static __always_inline bool should_filter_path(const char *path) {
    // Filter out high-frequency system paths to reduce overhead
    if (starts_with(path, "/proc/", FILENAME_LEN)) return true;
    if (starts_with(path, "/sys/", FILENAME_LEN)) return true;
    if (starts_with(path, "/dev/", FILENAME_LEN)) return true;
    if (starts_with(path, "/run/", FILENAME_LEN)) return true;
    if (starts_with(path, "/host/", FILENAME_LEN)) return true;

    // Check for cache directories
    #pragma unroll
    for (int i = 0; i < FILENAME_LEN - 8; i++) {
        if (path[i] == '\0') break;
        if (path[i] == '/' && path[i+1] == '.' && path[i+2] == 'c' &&
            path[i+3] == 'a' && path[i+4] == 'c' && path[i+5] == 'h' &&
            path[i+6] == 'e' && path[i+7] == '/') {
            return true;
        }
    }

    return false;
}

static __always_inline int fill_filename(struct file *file, struct event *e) {
    if (!file) {
        return -1;
    }

    u64 key = (u64)file;
    struct path_entry *cached = bpf_map_lookup_elem(&path_cache, &key);
    if (cached) {
        __builtin_memcpy(e->filename, cached->filename, FILENAME_LEN);
        e->filename[FILENAME_LEN - 1] = '\0';

        // Filter using cached full path
        if (should_filter_path(cached->filename)) {
            return -1;
        }
        return 0;
    }

    // Cache miss: fall back to dentry name only (basename)
    // Note: We cannot call bpf_d_path() from fentry context, so we can't
    // filter non-cached paths. The LSM hook should cache most file opens.
    struct dentry *dentry = BPF_CORE_READ(file, f_path.dentry);
    if (!dentry) {
        return -1;
    }

    const unsigned char *name = BPF_CORE_READ(dentry, d_name.name);
    unsigned int len = BPF_CORE_READ(dentry, d_name.len);

    if (!name) {
        return -1;
    }

    if (len >= FILENAME_LEN) {
        len = FILENAME_LEN - 1;
    }

    bpf_probe_read_kernel(&e->filename, len, name);
    e->filename[len] = '\0';
    return 0;
}

static __always_inline int submit_event_with_ret(struct file *file, ssize_t ret_val, u8 op) {
    if (!file) {
        return 0;
    }

    struct inode *inode = BPF_CORE_READ(file, f_inode);
    if (!inode) {
        return 0;
    }

    umode_t mode = BPF_CORE_READ(inode, i_mode);
    if (!S_ISREG(mode)) {
        return 0;
    }

    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) {
        return 0;
    }

    __builtin_memset(e, 0, sizeof(*e));

    u64 pid_tgid = bpf_get_current_pid_tgid();
    gid_t tgid = pid_tgid >> 32;
    pid_t pid = pid_tgid;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 ppid = BPF_CORE_READ(task, real_parent, tgid);

    if (tool_config.tgid && tgid != tool_config.tgid && ppid != tool_config.tgid) {
        bpf_ringbuf_discard(e, 0);
        return 0;
    }

    e->tgid = tgid;
    e->pid = pid;
    e->uid = bpf_get_current_uid_gid();
    e->ts = bpf_ktime_get_ns();
    e->size = ret_val >= 0 ? ret_val : 0;
    e->ret = ret_val;
    e->op = op;
    bpf_get_current_comm(e->comm, sizeof(e->comm));

    // Fill filename and apply kernel-side filtering
    int ret = fill_filename(file, e);
    if (ret < 0) {
        // Path is filtered out, discard the event
        bpf_ringbuf_discard(e, 0);
        return 0;
    }

    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("fexit/vfs_read")
int BPF_PROG(vfs_read_exit, struct file *file, char *buf, size_t count, loff_t *pos, ssize_t ret) {
    return submit_event_with_ret(file, ret, FILE_OP_READ);
}

SEC("fexit/vfs_write")
int BPF_PROG(vfs_write_exit, struct file *file, const char *buf, size_t count, loff_t *pos, ssize_t ret) {
    return submit_event_with_ret(file, ret, FILE_OP_WRITE);
}

SEC("lsm/file_open")
int BPF_PROG(vfs_file_open, struct file *file, const struct cred *cred) {
    if (!file) {
        return 0;
    }

    struct path_entry entry = {};
    unsigned long off = bpf_core_field_offset(struct file, f_path);
    struct path *path = (void *)file + off;

    long ret = bpf_d_path(path, entry.filename, sizeof(entry.filename));
    if (ret < 0) {
        return 0;
    }

    u64 key = (u64)file;
    bpf_map_update_elem(&path_cache, &key, &entry, BPF_ANY);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
