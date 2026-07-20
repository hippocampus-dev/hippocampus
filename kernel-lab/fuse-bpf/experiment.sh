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

config=(
  CONFIG_FUSE_FS=y
  CONFIG_BPF_SYSCALL=y
  CONFIG_BPF_JIT=y
  CONFIG_FUSE_BPF=y
  CONFIG_DEBUG_INFO_DWARF_TOOLCHAIN_DEFAULT=y
  CONFIG_DEBUG_INFO_BTF=y
)

source "${LAB}/lib.sh"
kernel_lab "$@"
