#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define BPF_FUSE_CONTINUE 0
#define BPF_FUSE_USER 1
#define BPF_FUSE_POSTFILTER 3
#define BPF_FUSE_USER_POSTFILTER 4
#define NAME_SCAN_MAX 256
#define SUFFIX_MAX 8
#define SUFFIX_LEN 4

#define S_IFMT 00170000
#define S_IFDIR 0040000

// The daemon sets this bit in the nodeid it replies with, and read_iter_prefilter tests it. No
// inode number reaches it, so the mark cannot be evicted, and a child that inherits its parent
// directory's nodeid inherits a clear bit rather than the parent's verdict.
#define SENSITIVE_TAG (1ULL << 63)

extern void bpf_fuse_get_ro_dynptr(const struct fuse_buffer *buffer,
                                   struct bpf_dynptr *dynptr) __ksym;
extern void *bpf_dynptr_slice(const struct bpf_dynptr *ptr, u32 offset, void *buffer,
                              u32 buffer__szk) __ksym;

const volatile struct {
    u8 suffixes[SUFFIX_MAX][SUFFIX_LEN];
    u32 suffixes_len;
} tool_config;

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
    // A failed lookup leaves out->attr zeroed, so nothing below may trust it.
    if (meta->error_in) {
        return BPF_FUSE_CONTINUE;
    }

    // Anything the daemon answers gets a nodeid from it; anything else keeps the 0 the backing
    // path left, which read_iter_prefilter reads as "not sensitive".
    // The daemon needs every directory on the way down to rebuild a nested path.
    if ((out->attr.mode & S_IFMT) == S_IFDIR) {
        return BPF_FUSE_USER_POSTFILTER;
    }
    if (name_is_sensitive(name)) {
        return BPF_FUSE_USER_POSTFILTER;
    }
    return BPF_FUSE_CONTINUE;
}

SEC("struct_ops/mask_mkdir_prefilter") u32 BPF_PROG(mask_mkdir_prefilter, const struct bpf_fuse_meta_info *meta, struct fuse_mkdir_in *in, struct fuse_buffer *name) {
    return BPF_FUSE_POSTFILTER;
}

SEC("struct_ops/mask_mkdir_postfilter") u32 BPF_PROG(mask_mkdir_postfilter, const struct bpf_fuse_meta_info *meta, const struct fuse_mkdir_in *in, const struct fuse_buffer *name) {
    if (meta->error_in) {
        return BPF_FUSE_CONTINUE;
    }
    // The new directory takes this parent's nodeid, so from here on the daemon cannot tell a
    // request in the parent from one in the child. It needs to know that.
    return BPF_FUSE_USER_POSTFILTER;
}

SEC("struct_ops/mask_create_open_prefilter") u32 BPF_PROG(mask_create_open_prefilter, const struct bpf_fuse_meta_info *meta, struct fuse_create_in *in, struct fuse_buffer *name) {
    return BPF_FUSE_POSTFILTER;
}

SEC("struct_ops/mask_create_open_postfilter") u32 BPF_PROG(mask_create_open_postfilter, const struct bpf_fuse_meta_info *meta, const struct fuse_create_in *in, const struct fuse_buffer *name, struct fuse_entry_out *entry_out, struct fuse_open_out *out) {
    if (meta->error_in) {
        return BPF_FUSE_CONTINUE;
    }
    // Creating through the mount fills neither nodeid nor attr, so only the daemon can find the
    // inode this name just got; hand it over so the file does not start out unmarked.
    if (name_is_sensitive(name)) {
        return BPF_FUSE_USER_POSTFILTER;
    }
    return BPF_FUSE_CONTINUE;
}

SEC("struct_ops/mask_read_iter_prefilter") u32 BPF_PROG(mask_read_iter_prefilter, const struct bpf_fuse_meta_info *meta, struct fuse_read_in *in) {
    if (meta->nodeid & SENSITIVE_TAG) {
        return BPF_FUSE_USER;
    }
    return BPF_FUSE_CONTINUE;
}

SEC(".struct_ops") struct fuse_ops mask_ops = {
    .lookup_prefilter = (void *)mask_lookup_prefilter,
    .lookup_postfilter = (void *)mask_lookup_postfilter,
    .mkdir_prefilter = (void *)mask_mkdir_prefilter,
    .mkdir_postfilter = (void *)mask_mkdir_postfilter,
    .create_open_prefilter = (void *)mask_create_open_prefilter,
    .create_open_postfilter = (void *)mask_create_open_postfilter,
    .read_iter_prefilter = (void *)mask_read_iter_prefilter,
    .name = "mask_ops",
};

char LICENSE[] SEC("license") = "GPL";
