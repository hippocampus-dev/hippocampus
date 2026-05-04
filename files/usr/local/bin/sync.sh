#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

mkdir -p ~/.local/share/fish
if [ -f ~/.local/share/fish/fish_history ] && [ $(stat -c %s ~/.local/share/fish/fish_history) -gt $(gcloud storage cat gs://kaidotio-sync/home/kai/.local/share/fish/fish_history | wc -c) ]; then
  gcloud storage cp ~/.local/share/fish/fish_history gs://kaidotio-sync/home/kai/.local/share/fish/fish_history
fi
gcloud storage cp gs://kaidotio-sync/home/kai/.local/share/fish/fish_history ~/.local/share/fish/fish_history

mkdir -p ~/.config/hippocampus/cluster
[ -f ~/.config/hippocampus/cluster/env.sh ] && gcloud storage cp ~/.config/hippocampus/cluster/env.sh gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh
gcloud storage cp gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh ~/.config/hippocampus/cluster/env.sh

mkdir -p ~/brain
opt=""
if [ $(find ~/brain -type f | wc -l) -ne 0 ]; then
  opt="--delete-unmatched-destination-objects"
fi
gcloud storage rsync ~/brain gs://kaidotio-sync/home/kai/brain --recursive $opt
gcloud storage rsync gs://kaidotio-sync/home/kai/brain ~/brain --recursive

mkdir -p ~/.secrets
opt=""
if [ $(find ~/.secrets -type f | wc -l) -ne 0 ]; then
  opt="--delete-unmatched-destination-objects"
fi
gcloud storage rsync ~/.secrets gs://kaidotio-sync/home/kai/.secrets --recursive $opt
gcloud storage rsync gs://kaidotio-sync/home/kai/.secrets ~/.secrets --recursive

mkdir -p ~/.vault
if [ -n "$RAILS_MASTER_KEY" ]; then
  t=/var/tmp/vault
  mkdir -p "$t"
  gcloud storage rsync gs://kaidotio-sync/home/kai/.vault "$t" --recursive
  while IFS= read -r file; do
    name=$(basename "$file")
    hash=$(sha256sum "$file" | cut -d ' ' -f 1)
    stored=$(cat "${t}/${name}.pgp.checksum" 2>/dev/null || :)
    if [ ! -f "${t}/${name}.pgp" ] || [ "$hash" != "$stored" ]; then
      gpg -cq --yes --batch --passphrase "$RAILS_MASTER_KEY" --output "${t}/${name}.pgp" "$file"
      echo "$hash" > "${t}/${name}.pgp.checksum"
    fi
  done < <(find ~/.vault -type f)
  while IFS= read -r file; do
    [ ! -f "${HOME}/.vault/$(basename "$file" .pgp)" ] && rm -f "$file" "${file}.checksum"
  done < <(find "$t" -type f -name '*.pgp')
  opt=""
  if [ $(find "$t" -type f -name '*.pgp' | wc -l) -ne 0 ]; then
    opt="--delete-unmatched-destination-objects"
  fi
  gcloud storage rsync "$t" gs://kaidotio-sync/home/kai/.vault --recursive $opt
  while IFS= read -r file; do
    tmp="${HOME}/.vault/$(basename "$file" .pgp).tmp"
    if gpg -dq --yes --batch --passphrase "$RAILS_MASTER_KEY" --output "$tmp" "$file"; then
      mv "$tmp" "${tmp%.tmp}"
    else
      rm -f "$tmp"
    fi
  done < <(find "$t" -type f -name '*.pgp')
fi
