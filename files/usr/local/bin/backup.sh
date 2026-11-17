#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly SOURCE="/"
readonly DESTINATION="/var/snapshots"
readonly RETENTION=1

mkdir -p "$DESTINATION"
btrfs subvolume snapshot -r "$SOURCE" "${DESTINATION}/$(date +%Y%m%d%H%M%S)"
find "$DESTINATION" -maxdepth 1 -mindepth 1 -type d | sort | head -n "-${RETENTION}" | xargs -r btrfs subvolume delete
