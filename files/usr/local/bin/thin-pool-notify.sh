#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

# dmeventd logs "WARNING: Thin pool vg0-thinpool-tpool data is now 88.18% full." as a pool fills
journalctl --follow --lines=0 --identifier=dmeventd --output=cat | while IFS= read -r line; do
  case "$line" in
    "WARNING: Thin pool "*" is now "*"% full.")
      notify-send --urgency=critical --hint="string:x-dunst-stack-tag:${line% is now *}" "Thin pool" "$line"
      ;;
  esac
done
