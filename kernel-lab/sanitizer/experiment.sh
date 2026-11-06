#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
readonly ENTRYPOINT
LAB=$(cd "${ENTRYPOINT}/.." && pwd)
readonly LAB

name="sanitizer"
tree_url="https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git"
# v7.1
base_commit="8cd9520d35a6c38db6567e97dd93b1f11f185dc6"
patches=()

config=(
  CONFIG_KASAN=y
  CONFIG_PROVE_LOCKING=y
  CONFIG_DEBUG_ATOMIC_SLEEP=y
  CONFIG_BPF_SYSCALL=y
  CONFIG_DEBUG_INFO_DWARF_TOOLCHAIN_DEFAULT=y
  CONFIG_DEBUG_INFO_BTF=y
)

source "${LAB}/lib.sh"
kernel_lab "$@"
