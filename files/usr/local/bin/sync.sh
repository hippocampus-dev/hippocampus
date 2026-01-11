#!/usr/bin/env -S bash -l

set -eo pipefail

mkdir -p ~/.local/share/fish
if [ -f ~/.local/share/fish/fish_history ] && [ $(stat -c %s ~/.local/share/fish/fish_history) -gt $(gcloud storage cat gs://kaidotio-sync/home/kai/.local/share/fish/fish_history | wc -c) ]; then
  gcloud storage cp ~/.local/share/fish/fish_history gs://kaidotio-sync/home/kai/.local/share/fish/fish_history
fi
gcloud storage cp gs://kaidotio-sync/home/kai/.local/share/fish/fish_history ~/.local/share/fish/fish_history

mkdir -p ~/.config/hippocampus/cluster
[ -f ~/.config/hippocampus/cluster/env.sh ] && gcloud storage cp ~/.config/hippocampus/cluster/env.sh gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh
gcloud storage cp gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh ~/.config/hippocampus/cluster/env.sh

opt=""
if [ $(find ~/.secrets -type f | wc -l) -ne 0 ]; then
  opt="--delete-unmatched-destination-objects"
fi
mkdir -p ~/.secrets
gcloud storage rsync ~/.secrets gs://kaidotio-sync/home/kai/.secrets --recursive $opt
gcloud storage rsync gs://kaidotio-sync/home/kai/.secrets ~/.secrets --recursive

opt=""
if [ $(find ~/.vault -type f | wc -l) -ne 0 ]; then
  opt="--delete-unmatched-destination-objects"
fi
mkdir -p ~/.vault
t=$(mktemp -d)
while IFS= read -r file; do
  gpg -cq --batch --passphrase "${RAILS_MASTER_KEY}" --output "${file}.pgp" "${file}"
  mv "${file}.pgp" "${t}/"
done < <(find ~/.vault -type f -not -name '*.pgp')
gcloud storage rsync "${t}" gs://kaidotio-sync/home/kai/.vault --recursive ${opt}
gcloud storage rsync gs://kaidotio-sync/home/kai/.vault "${t}" --recursive
while IFS= read -r file; do
  gpg -dq --batch --passphrase "${RAILS_MASTER_KEY}" --output "${file%.pgp}" "${file}"
  mv "${file%.pgp}" ~/.vault/
done < <(find "${t}" -type f -name '*.pgp')
