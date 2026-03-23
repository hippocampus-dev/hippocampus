#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

SRC="/"
DST="/var/snapshots"
RETENTION=1

mkdir -p $DST
btrfs subvolume snapshot -r $SRC ${DST}/$(date +%Y%m%d%H%M%S)
find $DST -maxdepth 1 -mindepth 1 -type d | sort | head -n -${RETENTION} | xargs -r btrfs subvolume delete
