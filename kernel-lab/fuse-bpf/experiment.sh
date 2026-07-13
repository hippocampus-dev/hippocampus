#!/usr/bin/env bash

# -E (errtrace) so the ERR trap fires inside lib.sh's sourced functions, where all
# the real work runs; plain set -e would abort but print no diagnostic.
set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LAB="$(cd "${HERE}/.." && pwd)"

name="fuse-bpf"
tree_url="https://git.kernel.org/pub/scm/linux/kernel/git/bpf/bpf-next.git"
base_commit="e63985ecd22681c7f5975f2e8637187a326b6791"
patches=("${HERE}/fuse-bpf-v4.mbox")

# defconfig already provides virtio-blk/net + ext4 (=y); only fuse-bpf's options are
# added. BPF_JIT is required — fuse-bpf's struct_ops references BPF_MODULE_OWNER.
config=(
  CONFIG_FUSE_FS=y
  CONFIG_BPF_SYSCALL=y
  CONFIG_BPF_JIT=y
  CONFIG_FUSE_BPF=y
)

source "${LAB}/lib.sh"
kernel_lab "$@"
