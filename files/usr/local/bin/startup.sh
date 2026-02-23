#!/usr/bin/env -S bash -l

set -e

sudo pacman-key --refresh || true

bash /opt/hippocampus/bin/decrypt.sh
