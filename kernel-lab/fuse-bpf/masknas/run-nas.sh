#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
BACKING_DIR=/srv/nas-backing
MOUNT_DIR=/srv/nas
BINARY="${ENTRYPOINT}/target/release/masknas"
LOG=/tmp/masknas.log

function usage() {
  cat <<EOS
Usage:
   run-nas.sh {up|down|build}

up      provision, build, seed data, mount, and launch the daemon
down    stop the daemon and unmount
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

  sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c | sudo tee /usr/include/vmlinux.h >/dev/null
}

build() {
  # shellcheck disable=SC1090
  [ -f ~/.cargo/env ] && . ~/.cargo/env
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
}

down() {
  if daemon_running; then
    sudo pkill -x masknas || true
  fi

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

case "${1:-}" in
  up) up ;;
  down) down ;;
  build) build ;;
  -h|--help) usage ;;
  *) usage; exit 1 ;;
esac
