#!/usr/bin/env bash

set -e

if removes=$(pacman -Qqdt); then
  sudo pacman -Rns --noconfirm $removes
fi

sudo pacman -Syu --noconfirm
docker system prune --all --volumes --force

sudo paccache -rk1

find /opt/hippocampus -name "target" -type d | while IFS= read -r target; do
  if [ -d "${target}/debug/incremental" ]; then
    find "${target}/debug/incremental" -maxdepth 1 -type d -atime +7 -exec rm -rf {} +
  fi
  if [ -d "${target}/release/incremental" ]; then
    find "${target}/release/incremental" -maxdepth 1 -type d -atime +7 -exec rm -rf {} +
  fi
done

if [ -d ~/.cache/uv/archive-v0 ]; then
  find ~/.cache/uv/archive-v0 -maxdepth 1 -type d -atime +30 -exec rm -rf {} +
fi
if [ -d ~/.cache/go-build ]; then
  find ~/.cache/go-build -type f -atime +30 -delete
fi
if [ -d ~/.npm/_cacache ]; then
  find ~/.npm/_cacache -type f -atime +30 -delete
fi
if [ -d ~/.bun/install ]; then
  find ~/.bun/install -type f -atime +30 -delete
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

find /usr/local/src -mindepth 1 -maxdepth 1 -type d | while IFS= read -r directory; do
  (
    revision="$(basename ${directory}).rev"

    cd ${directory}
    git pull
    current_revision=$(git rev-parse HEAD)
    if [ ! -f $revision ] || [ "$(cat $revision)" != "$current_revision" ]; then
      git clean -fd
      makepkg -si --noconfirm || true
      echo $current_revision > $revision
    fi
  )
done
