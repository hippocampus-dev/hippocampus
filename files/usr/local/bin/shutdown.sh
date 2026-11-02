#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

RETENTION_DAYS=7

if removes=$(pacman -Qqdt); then
  sudo pacman -Rns --noconfirm $removes
fi

sudo pacman-key --init
sudo pacman-key --populate archlinux
sudo pacman -Sy --noconfirm archlinux-keyring
sudo pacman -Su --noconfirm
docker system prune --volumes --force

sudo paccache -rk1

# kernel-lab/.build holds kernel source directories named target, which carry no CACHEDIR.TAG
while IFS= read -r target; do
  find "$target" ! -name CACHEDIR.TAG -type f -atime "+${RETENTION_DAYS}" -delete
  find "$target" -mindepth 1 -depth -type d -empty -delete
done < <(find /opt/hippocampus -path "*/target/CACHEDIR.TAG" -printf '%h\n')

if [ -d ~/.cache/uv/archive-v0 ]; then
  find ~/.cache/uv/archive-v0 -maxdepth 1 -type d -atime "+${RETENTION_DAYS}" -exec rm -rf {} +
fi
if [ -d ~/.cache/go-build ]; then
  find ~/.cache/go-build -type f -atime "+${RETENTION_DAYS}" -delete
fi
if [ -d ~/.npm/_cacache ]; then
  find ~/.npm/_cacache -type f -atime "+${RETENTION_DAYS}" -delete
fi
if [ -d ~/.bun/install ]; then
  find ~/.bun/install -type f -atime "+${RETENTION_DAYS}" -delete
fi

if [ -d ~/.gradle/caches ]; then
  find ~/.gradle/caches -maxdepth 1 -type d -name '[0-9]*' | sort -V | head -n -1 | xargs -r rm -rf
fi

if [ -d ~/.cache/JetBrains ]; then
  for product in IntelliJIdea AndroidStudio; do
    find ~/.cache/JetBrains -maxdepth 1 -type d -name "${product}*" | sort -V | head -n -1 | xargs -r rm -rf
  done
fi

if [ -d ~/.cache/ms-playwright ]; then
  for browser in chromium chromium_headless_shell firefox webkit; do
    find ~/.cache/ms-playwright -maxdepth 1 -type d -name "${browser}-*" | sort -V | head -n -1 | xargs -r rm -rf
  done
fi

while IFS= read -r directory; do
  (
    revision="$(basename "$directory").rev"

    cd "$directory"
    if git rev-parse --abbrev-ref --symbolic-full-name @{u} > /dev/null 2>&1; then
      git pull
    fi
    current_revision=$(git rev-parse HEAD)
    if [ ! -f "$revision" ] || [ "$(cat "$revision")" != "$current_revision" ]; then
      git clean -fd
      makepkg -si --noconfirm || true
      echo "$current_revision" > "$revision"
    fi
  )
done < <(find /usr/local/src -mindepth 1 -maxdepth 1 -type d)
