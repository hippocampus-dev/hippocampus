#!/usr/bin/env -S bash -l

set -e

sudo pacman-key --refresh

bash /opt/hippocampus/bin/decrypt.sh
