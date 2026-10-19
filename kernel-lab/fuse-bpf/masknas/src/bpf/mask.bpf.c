#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define BPF_FUSE_CONTINUE 0
#define BPF_FUSE_USER 1
#define BPF_FUSE_POSTFILTER 3
#define BPF_FUSE_USER_POSTFILTER 4
#define SUFFIX_MAX 8
#define SUFFIX_LEN_MAX 8

#define S_IFMT 00170000
#define S_IFDIR 0040000

#define SENSITIVE_TAG (1ULL << 63)

extern u32 bpf_fuse_return_len(struct fuse_buffer *buffer) __ksym;
extern void bpf_fuse_get_ro_dynptr(const struct fuse_buffer *buffer,
                                   struct bpf_dynptr *dynptr) __ksym;
extern void *bpf_dynptr_slice(const struct bpf_dynptr *ptr, u32 offset, void *buffer,
                              u32 buffer__szk) __ksym;

const volatile struct {
    u8 suffixes[SUFFIX_MAX][SUFFIX_LEN_MAX];
    u8 suffix_sizes[SUFFIX_MAX];
    u32 suffixes_len;
} tool_config;

static __always_inline bool name_is_sensitive(const struct fuse_buffer *name) {
    struct bpf_dynptr name_ptr;
    u8 window[SUFFIX_LEN_MAX] = {};

    u32 length = bpf_fuse_return_len((struct fuse_buffer *)name) - 1;

    bpf_fuse_get_ro_dynptr(name, &name_ptr);
    for (int i = 0; i < SUFFIX_LEN_MAX; i++) {
        if (i >= length) {
            break;
        }
        u8 *byte = bpf_dynptr_slice(&name_ptr, length - 1 - i, NULL, 1);
        if (!byte) {
            return false;
        }
        window[SUFFIX_LEN_MAX - 1 - i] = *byte;
    }

    for (int i = 0; i < tool_config.suffixes_len; i++) {
        if (length < tool_config.suffix_sizes[i]) {
            continue;
        }
        bool m = true;
        for (int j = 0; j < SUFFIX_LEN_MAX; j++) {
            if (j + tool_config.suffix_sizes[i] < SUFFIX_LEN_MAX) {
                continue;
            }
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
    if (meta->error_in) {
        return BPF_FUSE_CONTINUE;
    }

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
    return BPF_FUSE_USER_POSTFILTER;
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
    .read_iter_prefilter = (void *)mask_read_iter_prefilter,
    .name = "mask_ops",
};

char LICENSE[] SEC("license") = "GPL";
