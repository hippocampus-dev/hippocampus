#!/usr/bin/env -S bash -l

set -e
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

sudo pacman-key --refresh || true

bash /opt/hippocampus/bin/decrypt.sh
