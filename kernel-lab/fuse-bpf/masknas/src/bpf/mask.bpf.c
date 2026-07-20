#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define BPF_FUSE_CONTINUE 0
#define BPF_FUSE_USER 1
#define BPF_FUSE_POSTFILTER 3
#define BPF_FUSE_USER_POSTFILTER 4
#define NAME_SCAN_MAX 256

extern void bpf_fuse_get_ro_dynptr(const struct fuse_buffer *buffer,
                                   struct bpf_dynptr *dynptr) __ksym;
extern void *bpf_dynptr_slice(const struct bpf_dynptr *ptr, u32 offset, void *buffer,
                              u32 buffer__szk) __ksym;

SEC(".maps") struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u64);
    __type(value, __u8);
    __uint(max_entries, 4096);
} sensitive_nodes;

// Suffix match via a 4-byte rolling window, so the verifier only sees a
// constant 1-byte dynptr slice at a bounded offset.
static __always_inline bool name_is_sensitive(const struct fuse_buffer *name) {
    struct bpf_dynptr name_ptr;
    char a = 0, b = 0, c = 0, d = 0;
    int i;

    bpf_fuse_get_ro_dynptr(name, &name_ptr);

    for (i = 0; i < NAME_SCAN_MAX; i++) {
        char *p = bpf_dynptr_slice(&name_ptr, i, NULL, 1);

        if (!p || *p == '\0') {
            break;
        }
        a = b;
        b = c;
        c = d;
        d = *p;
    }

    if (a == '.' && b == 'c' && c == 's' && d == 'v') {
        return true;
    }
    if (a == '.' && b == 't' && c == 'x' && d == 't') {
        return true;
    }
    return false;
}

SEC("struct_ops/mask_lookup_prefilter") u32 BPF_PROG(mask_lookup_prefilter, const struct bpf_fuse_meta_info *meta, struct fuse_buffer *name) {
    return BPF_FUSE_POSTFILTER;
}

SEC("struct_ops/mask_lookup_postfilter") u32 BPF_PROG(mask_lookup_postfilter, const struct bpf_fuse_meta_info *meta, const struct fuse_buffer *name, struct fuse_entry_out *out, struct fuse_buffer *entries) {
    __u64 nodeid = out->nodeid;
    __u8 one = 1;

    if (!name_is_sensitive(name)) {
        bpf_map_delete_elem(&sensitive_nodes, &nodeid);
        return BPF_FUSE_CONTINUE;
    }
    bpf_map_update_elem(&sensitive_nodes, &nodeid, &one, BPF_ANY);
    bpf_printk("mask: sensitive nodeid=%llu", nodeid);
    return BPF_FUSE_USER_POSTFILTER;
}

SEC("struct_ops/mask_read_iter_prefilter") u32 BPF_PROG(mask_read_iter_prefilter, const struct bpf_fuse_meta_info *meta, struct fuse_read_in *in) {
    __u64 nodeid = meta->nodeid;

    if (bpf_map_lookup_elem(&sensitive_nodes, &nodeid)) {
        bpf_printk("mask: read USER nodeid=%llu", nodeid);
        return BPF_FUSE_USER;
    }
    bpf_printk("mask: read CONTINUE nodeid=%llu", nodeid);
    return BPF_FUSE_CONTINUE;
}

SEC(".struct_ops") struct fuse_ops mask_ops = {
    .lookup_prefilter = (void *)mask_lookup_prefilter,
    .lookup_postfilter = (void *)mask_lookup_postfilter,
    .read_iter_prefilter = (void *)mask_read_iter_prefilter,
    .name = "mask_ops",
};

char LICENSE[] SEC("license") = "GPL";
