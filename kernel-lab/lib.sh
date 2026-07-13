#!/usr/bin/env bash
# Sourced library (no set -e / trap of its own; the experiment script owns those).
# Shared machinery for kernel-lab experiments. An experiment script sets:
#   name         short id, also the artifact subdirectory under .build/
#   tree_url     git kernel tree to fetch
#   base_commit  pinned commit the patches/config target
#   patches      array of mbox files to git am (may be empty)
#   config       array of CONFIG_*=y lines appended onto defconfig
# then: source "${LAB}/lib.sh"; kernel_lab "$@"

kernel_lab_usage() {
  cat <<EOS
Usage:
   ${0##*/} {build|rootfs|run}

build   fetch ${base_commit}, apply patches, build a bzImage
rootfs  download the Debian cloud image and build an SSH seed ISO
run     boot the VM (KVM if available, else TCG); Ctrl-C stops it

Env: JOBS (build parallelism, default: nproc)
EOS
}

kernel_lab_build() {
  local work="${LAB}/.build/${name}"
  local source_tree="${work}/linux"
  export ARCH=x86_64

  # Force -std=gnu11: GCC >= 15 defaults to gnu23, which older trees fail to build.
  # Keep the wrapper in the lab, not $TMPDIR, in case /tmp is mounted noexec.
  local wrapper="${work}/.gcc-wrapper"
  mkdir -p "${wrapper}"
  cat > "${wrapper}/gcc" <<'WRAP'
#!/bin/sh
exec /usr/bin/gcc -std=gnu11 "$@"
WRAP
  chmod +x "${wrapper}/gcc"
  export PATH="${wrapper}:${PATH}"

  # Re-fetch/re-patch only when the pinned commit or patch set changes; the sentinel
  # stores their fingerprint. (It also guards against a wedged partial checkout: git
  # init creates .git before fetch/am, so .git existence alone is not a safe gate.)
  local patch_hash=""
  if [ "${#patches[@]}" -gt 0 ]; then
    patch_hash="$(cat "${patches[@]}" | sha256sum | cut -d ' ' -f 1)"
  fi
  local fingerprint="${base_commit}:${patch_hash}"
  if [ "$(cat "${source_tree}/.prepared" 2>/dev/null)" != "${fingerprint}" ]; then
    rm -rf "${source_tree}"
    mkdir -p "${source_tree}"
    git -C "${source_tree}" init -q
    git -C "${source_tree}" remote add origin "${tree_url}"
    git -C "${source_tree}" fetch --depth 1 origin "${base_commit}"
    git -C "${source_tree}" checkout -q FETCH_HEAD
    git -C "${source_tree}" config commit.gpgsign false
    git -C "${source_tree}" config user.email lab@example.com
    git -C "${source_tree}" config user.name lab
    local patch
    for patch in "${patches[@]}"; do
      git -C "${source_tree}" am "${patch}"
    done
    echo "${fingerprint}" > "${source_tree}/.prepared"
  fi

  make -C "${source_tree}" -s defconfig
  if [ "${#config[@]}" -gt 0 ]; then
    printf '%s\n' "${config[@]}" >> "${source_tree}/.config"
  fi
  make -C "${source_tree}" -s olddefconfig

  local line option
  for line in "${config[@]}"; do
    case "${line}" in
      *=y)
        option="${line%=y}"
        if ! grep -q "^${option}=y" "${source_tree}/.config"; then
          echo "requested ${option}=y was not enabled (unmet dependency?)" >&2
          exit 1
        fi
        ;;
    esac
  done

  make -C "${source_tree}" -j"${JOBS:-$(nproc)}" bzImage
  echo "built: ${source_tree}/arch/x86/boot/bzImage"
}

kernel_lab_rootfs() {
  local work="${LAB}/.build/${name}"
  local image_url="https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2"
  mkdir -p "${work}/images"

  if [ ! -f "${work}/images/rootfs.qcow2" ]; then
    wget -q -O "${work}/images/rootfs.qcow2.part" "${image_url}"
    mv "${work}/images/rootfs.qcow2.part" "${work}/images/rootfs.qcow2"
  fi

  if [ ! -f "${work}/id_ed25519" ] || [ ! -f "${work}/id_ed25519.pub" ]; then
    ssh-keygen -t ed25519 -N '' -C "${name}-lab" -f "${work}/id_ed25519"
  fi

  local seed public_key
  seed="$(mktemp -d)"
  trap 'rm -rf "${seed}"' EXIT
  public_key="$(cat "${work}/id_ed25519.pub")"
  cat > "${seed}/meta-data" <<EOF
instance-id: ${name}-lab
local-hostname: ${name}-lab
EOF
  cat > "${seed}/user-data" <<EOF
#cloud-config
hostname: ${name}-lab
users:
  - name: debian
    groups: [sudo]
    sudo: "ALL=(ALL) NOPASSWD:ALL"
    shell: /bin/bash
    lock_passwd: false
    ssh_authorized_keys:
      - ${public_key}
ssh_pwauth: false
chpasswd:
  expire: false
  users:
    - name: debian
      password: lab
      type: text
EOF
  xorriso -as mkisofs -output "${work}/images/seed.iso" -volid CIDATA -joliet -rock \
    "${seed}/user-data" "${seed}/meta-data" 2>/dev/null
  echo "built: ${work}/images/rootfs.qcow2, ${work}/images/seed.iso"
}

kernel_lab_run() {
  local work="${LAB}/.build/${name}"
  local bzimage="${work}/linux/arch/x86/boot/bzImage"
  local rootfs="${work}/images/rootfs.qcow2"
  local seed="${work}/images/seed.iso"

  local file
  for file in "${bzimage}" "${rootfs}" "${seed}"; do
    if [ ! -f "${file}" ]; then
      echo "missing: ${file}" >&2
      echo "run '${0##*/} build' and '${0##*/} rootfs' first" >&2
      exit 1
    fi
  done

  local acceleration processors accelerators
  accelerators="$(qemu-system-x86_64 -accel help 2>/dev/null || true)"
  if grep -qw kvm <<< "${accelerators}" && [ -w /dev/kvm ]; then
    acceleration=(-accel kvm -cpu host)
    processors=4
  else
    acceleration=(-accel tcg -cpu max)
    processors=2
  fi

  # stdin from /dev/null: the interface is SSH, not the serial console.
  qemu-system-x86_64 \
    -name "${name}-lab" \
    -machine q35 "${acceleration[@]}" -smp "${processors}" -m 4096 \
    -kernel "${bzimage}" \
    -append "root=/dev/vda1 rw console=ttyS0 net.ifnames=0 systemd.show_status=1" \
    -drive file="${rootfs}",if=virtio,format=qcow2 \
    -drive file="${seed}",if=virtio,format=raw,readonly=on \
    -netdev user,id=n0,hostfwd=tcp::2222-:22 \
    -device virtio-net-pci,netdev=n0 \
    -nographic < /dev/null &
  local qemu_pid=$!
  trap "kill ${qemu_pid} 2>/dev/null" EXIT INT TERM

  echo "${name} VM up (pid ${qemu_pid})."
  echo "SSH:  ssh -p 2222 -i ${work}/id_ed25519 debian@localhost"
  echo "Stop: Ctrl-C here, or close this terminal."
  wait "${qemu_pid}" || true
}

kernel_lab() {
  case "${1:-}" in
    -h|--help) kernel_lab_usage; exit 0 ;;
  esac

  local required
  for required in name tree_url base_commit; do
    if [ -z "${!required}" ]; then
      echo "experiment script must set '${required}'" >&2
      exit 1
    fi
  done

  case "${1:-}" in
    build) kernel_lab_build ;;
    rootfs) kernel_lab_rootfs ;;
    run) kernel_lab_run ;;
    *) kernel_lab_usage; exit 1 ;;
  esac
}
