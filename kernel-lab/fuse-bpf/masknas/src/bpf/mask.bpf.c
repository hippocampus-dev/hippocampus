#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define BPF_FUSE_CONTINUE 0
#define BPF_FUSE_USER 1
#define BPF_FUSE_POSTFILTER 3
#define BPF_FUSE_USER_POSTFILTER 4
#define MAX_ENTRIES 4096
#define NAME_SCAN_MAX 256
#define SUFFIX_MAX 8
#define SUFFIX_LEN 4

extern void bpf_fuse_get_ro_dynptr(const struct fuse_buffer *buffer,
                                   struct bpf_dynptr *dynptr) __ksym;
extern void *bpf_dynptr_slice(const struct bpf_dynptr *ptr, u32 offset, void *buffer,
                              u32 buffer__szk) __ksym;

const volatile struct {
    u8 suffixes[SUFFIX_MAX][SUFFIX_LEN];
    u32 suffixes_len;
} tool_config;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);
    __type(value, __u8);
} sensitive_nodes;

// Suffix match via a 4-byte rolling window, so the verifier only sees a
// constant 1-byte dynptr slice at a bounded offset.
static __always_inline bool name_is_sensitive(const struct fuse_buffer *name) {
    struct bpf_dynptr name_ptr;
    u8 window[SUFFIX_LEN] = {0};

    bpf_fuse_get_ro_dynptr(name, &name_ptr);

    for (int i = 0; i < NAME_SCAN_MAX; i++) {
        char *p = bpf_dynptr_slice(&name_ptr, i, NULL, 1);

        if (!p || *p == '\0') {
            break;
        }
        window[0] = window[1];
        window[1] = window[2];
        window[2] = window[3];
        window[3] = *p;
    }

    for (int i = 0; i < tool_config.suffixes_len; i++) {
        bool m = true;
        for (int j = 0; j < SUFFIX_LEN; j++) {
            if (window[j] != tool_config.suffixes[i][j]) {
                m = false;
            }
        }
        if (m) {
            return true;
        }
    }
    return false;
}

SEC("struct_ops/mask_lookup_prefilter") u32 BPF_PROG(mask_lookup_prefilter, const struct bpf_fuse_meta_info *meta, struct fuse_buffer *name) {
    return BPF_FUSE_POSTFILTER;
}

SEC("struct_ops/mask_lookup_postfilter") u32 BPF_PROG(mask_lookup_postfilter, const struct bpf_fuse_meta_info *meta, const struct fuse_buffer *name, struct fuse_entry_out *out, struct fuse_buffer *entries) {
    u64 nodeid = out->nodeid;
    u8 one = 1;

    if (!name_is_sensitive(name)) {
        bpf_map_delete_elem(&sensitive_nodes, &nodeid);
        return BPF_FUSE_CONTINUE;
    }
    bpf_map_update_elem(&sensitive_nodes, &nodeid, &one, BPF_ANY);
    return BPF_FUSE_USER_POSTFILTER;
}

SEC("struct_ops/mask_read_iter_prefilter") u32 BPF_PROG(mask_read_iter_prefilter, const struct bpf_fuse_meta_info *meta, struct fuse_read_in *in) {
    u64 nodeid = meta->nodeid;

    if (bpf_map_lookup_elem(&sensitive_nodes, &nodeid)) {
        return BPF_FUSE_USER;
    }
    return BPF_FUSE_CONTINUE;
}

SEC(".struct_ops") struct fuse_ops mask_ops = {
    .lookup_prefilter = (void *)mask_lookup_prefilter,
    .lookup_postfilter = (void *)mask_lookup_postfilter,
    .read_iter_prefilter = (void *)mask_read_iter_prefilter,
    .name = "mask_ops",
};

char LICENSE[] SEC("license") = "GPL";
