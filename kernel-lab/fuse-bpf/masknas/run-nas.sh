#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# Mirrors the binary's defaults; up() passes both explicitly anyway.
BACKING_DIR=/srv/nas-backing
MOUNT_DIR=/srv/nas
BINARY="${ENTRYPOINT}/target/release/masknas"
LOG=/tmp/masknas.log

function usage() {
  cat <<EOS
Usage:
   run-nas.sh {up|down|demo|build}

up      provision, build, seed demo data, mount, and launch the daemon
down    stop the daemon and unmount
demo    show the masked view, the plaintext backing, and the silent fast path
build   cargo build only (after a prior up)
EOS
}

provision() {
  local packages=(clang llvm gcc libbpf-dev bpftool pkg-config libelf-dev zlib1g-dev make curl ca-certificates)
  local missing=()
  local package
  for package in "${packages[@]}"; do
    if ! dpkg -s "${package}" >/dev/null 2>&1; then
      missing+=("${package}")
    fi
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    sudo apt-get update -y
    sudo apt-get install -y "${missing[@]}"
  fi

  if ! command -v cargo >/dev/null 2>&1; then
    curl --proto '=https' --tlsv1.2 -sSf \
      https://sh.rustup.rs | sh -s -- -y --profile minimal
  fi

  # struct_ops needs vmlinux BTF; emit it like the sibling Dockerfiles.
  sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c | sudo tee /usr/include/vmlinux.h >/dev/null
}

build() {
  # shellcheck disable=SC1090
  [ -f ~/.cargo/env ] && . ~/.cargo/env
  # libbpf-cargo needs rustfmt, which the minimal rustup profile omits.
  ( cd "${ENTRYPOINT}" && rustup component add rustfmt && cargo build --release )
}

seed_assets() {
  sudo mkdir -p "${BACKING_DIR}" "${MOUNT_DIR}"

  sudo tee "${BACKING_DIR}/customers.csv" >/dev/null <<'EOS'
id,name,email,phone,card
1,Taro Yamada,taro@example.com,090-1234-5678,4111 1111 1111 1111
2,Hanako Suzuki,hanako.suzuki@example.co.jp,03-1234-5678,5500-0000-0000-0004
3,Jiro Tanaka,jiro@example.org,+81-90-8765-4321,340000000000009
EOS

  sudo tee "${BACKING_DIR}/notes.txt" >/dev/null <<'EOS'
Reminder: email the report to ops@example.com.
Escalation hotline is 0120-000-000; personal cell 080-9999-1111.
Test charge on card 4242 4242 4242 4242 should be refunded.
EOS

  # Non-target payload: the NAS "main traffic" that must stay on the fast path.
  if [ ! -f "${BACKING_DIR}/movie.bin" ]; then
    sudo dd if=/dev/urandom of="${BACKING_DIR}/movie.bin" bs=1M count=64 status=none
  fi
}

daemon_running() {
  pgrep -x masknas >/dev/null 2>&1
}

up() {
  provision
  build
  seed_assets

  if daemon_running; then
    echo "masknas already running" >&2
    return 0
  fi

  sudo nohup "${BINARY}" --backing-directory "${BACKING_DIR}" --mount-directory "${MOUNT_DIR}" >"${LOG}" 2>&1 &

  # The mount is owned by root (user_id=0), so probe it as root.
  local waited=0
  while ! sudo mountpoint -q "${MOUNT_DIR}"; do
    if [ "${waited}" -ge 50 ]; then
      echo "mount did not come up; see ${LOG}" >&2
      sudo cat "${LOG}" >&2 || true
      exit 1
    fi
    sleep 0.1
    waited=$((waited + 1))
  done

  echo "NAS up: ${MOUNT_DIR} (backing ${BACKING_DIR}); daemon log: ${LOG}"
  echo "Try: run-nas.sh demo"
}

down() {
  if daemon_running; then
    sudo pkill -x masknas || true
  fi

  # The daemon unmounts from its own drop guard, so let it exit before probing.
  local waited=0
  while daemon_running && [ "${waited}" -lt 50 ]; do
    sleep 0.1
    waited=$((waited + 1))
  done
  if daemon_running; then
    echo "masknas did not exit; see ${LOG}" >&2
    sudo pkill -KILL -x masknas || true
  fi

  if sudo mountpoint -q "${MOUNT_DIR}"; then
    sudo umount "${MOUNT_DIR}" || sudo umount -l "${MOUNT_DIR}"
  fi
  echo "NAS down"
}

demo() {
  # The masked view is a root-owned FUSE mount, so read it as root.
  echo "== masked view (customers.csv) =="
  sudo cat "${MOUNT_DIR}/customers.csv"
  echo
  echo "== masked view (notes.txt) =="
  sudo cat "${MOUNT_DIR}/notes.txt"
  echo
  echo "== backing is untouched plaintext (proves masking is a view) =="
  sudo cat "${BACKING_DIR}/customers.csv"
  echo
  echo "== non-target read stays on the kernel backing path (daemon is silent) =="
  sudo cat "${MOUNT_DIR}/movie.bin" > /dev/null
  echo "read 64MiB movie.bin; see ${LOG} — no read line for it"
  echo
  echo "== daemon log =="
  sudo cat "${LOG}"
}

case "${1:-}" in
  up) up ;;
  down) down ;;
  demo) demo ;;
  build) build ;;
  -h|--help) usage ;;
  *) usage; exit 1 ;;
esac
