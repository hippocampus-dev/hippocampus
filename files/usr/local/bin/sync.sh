#!/usr/bin/env bash

set -eo pipefail

mkdir -p /home/kai/.local/share/fish
if [ -e /home/kai/.local/share/fish/fish_history ] && [ $(stat -c %s /home/kai/.local/share/fish/fish_history) -gt $(gsutil cp gs://kaidotio-sync/home/kai/.local/share/fish/fish_history - | wc -c) ]; then
  gsutil cp /home/kai/.local/share/fish/fish_history gs://kaidotio-sync/home/kai/.local/share/fish/fish_history
fi
gsutil cp gs://kaidotio-sync/home/kai/.local/share/fish/fish_history /home/kai/.local/share/fish/fish_history

mkdir -p /home/kai/.local/share/gvfs-metadata
gsutil -m rsync -r /home/kai/.local/share/gvfs-metadata gs://kaidotio-sync/home/kai/.local/share/gvfs-metadata
gsutil -m rsync -r gs://kaidotio-sync/home/kai/.local/share/gvfs-metadata /home/kai/.local/share/gvfs-metadata

mkdir -p /home/kai/.secrets
gsutil -m rsync -r /home/kai/.secrets gs://kaidotio-sync/home/kai/.secrets
gsutil -m rsync -r gs://kaidotio-sync/home/kai/.secrets /home/kai/.secrets

mkdir -p /home/kai/.config/hippocampus/cluster
[ -e /home/kai/.config/hippocampus/cluster/env.sh ] && gsutil cp /home/kai/.config/hippocampus/cluster/env.sh gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh
gsutil cp gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh /home/kai/.config/hippocampus/cluster/env.sh
