#include "vmlinux.h"
#include <bpf/bpf_helpers.h>

static __always_inline u64 log2(u32 v) {
    u32 shift, r;

    r = (v > 0xFFFF) << 4; v >>= r;
    shift = (v > 0xFF) << 3; v >>= shift; r |= shift;
    shift = (v > 0xF) << 2; v >>= shift; r |= shift;
    shift = (v > 0x3) << 1; v >>= shift; r |= shift;
    r |= (v >> 1);

    return r;
}

static __always_inline u64 log2l(u64 v) {
    u32 hi = v >> 32;

    if (hi) {
        return log2(hi) + 32;
    } else {
        return log2(v);
    }
}

#define min(x, y) ({ typeof(x) _min1 = (x); typeof(y) _min2 = (y); (void) (&_min1 == &_min2); _min1 < _min2 ? _min1 : _min2; })
