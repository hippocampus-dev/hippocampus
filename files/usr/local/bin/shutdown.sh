#!/usr/bin/env bash

set -eo pipefail

f=$(mktemp)
pacman -Syu --noconfirm >> $f

find /usr/local/src -mindepth 1 -maxdepth 1 -type d | while IFS= read -r directory; do
  (
    cd $directory
    sudo -u kai git pull >> $f
    sudo -u kai makepkg -si --noconfirm || true >> $f
  )
done

cat $f > /var/log/shutdown
