#!/usr/bin/env bash

set -e

if removes=$(pacman -Qqdt); then
  sudo pacman -Rns --noconfirm $removes
fi

sudo pacman -Syu --noconfirm
docker system prune --all --volumes --force

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
