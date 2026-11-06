#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

strip_metadata() {
  case "${1,,}" in
    *.mp4 | *.mov | *.m4v | *.mkv | *.webm | *.avi)
      # GoPro GPMF data tracks carry GPS
      temporary=$(mktemp --suffix=".${1##*.}" ~/.vault-strip.XXXXXX)
      trap 'rm -f "$temporary"' EXIT
      if ffmpeg -nostdin -loglevel error -y -i "$1" -map 0 -map -0:d -map_metadata -1 -map_chapters -1 -c copy -fflags +bitexact "$temporary"; then
        chmod --reference="$1" "$temporary"
        touch --reference="$1" "$temporary"
        mv "$temporary" "$1"
      else
        rm -f "$temporary"
        echo "keep metadata in ${1}: ffmpeg could not remux it" 1>&2
      fi
      ;;
    *.jpg | *.jpeg | *.png | *.webp | *.heic | *.heif)
      # Without Orientation and the ICC profile a photo displays rotated or in the wrong colors
      if ! exiftool -q -q -all= -tagsFromFile @ -Orientation -ICC_Profile -P -overwrite_original_in_place "$1"; then
        echo "keep metadata in ${1}: exiftool could not rewrite it" 1>&2
      fi
      ;;
  esac
}

# rclone.conf would keep the key on disk
vault() {
  RCLONE_CONFIG_VAULTGCS_TYPE="google cloud storage" \
    RCLONE_CONFIG_VAULTGCS_ENV_AUTH=true \
    RCLONE_CONFIG_VAULTGCS_BUCKET_POLICY_ONLY=true \
    RCLONE_CONFIG_VAULT_TYPE=crypt \
    RCLONE_CONFIG_VAULT_REMOTE=vaultgcs:kaidotio-sync/home/kai/.vault \
    RCLONE_CONFIG_VAULT_PASSWORD="$password" \
    rclone "$@"
}

mkdir -p ~/.local/share/fish
touch ~/.local/share/fish/fish_history
remote=$(gcloud storage cat gs://kaidotio-sync/home/kai/.local/share/fish/fish_history)
if [ "$(awk 'NR <= 2' ~/.local/share/fish/fish_history)" != "$(awk 'NR <= 2' <<< "$remote")" ]; then
  { printf '%s\n' "$remote"; cat ~/.local/share/fish/fish_history; } > ~/.local/share/fish/fish_history.tmp
  mv ~/.local/share/fish/fish_history.tmp ~/.local/share/fish/fish_history
fi
gcloud storage cp ~/.local/share/fish/fish_history gs://kaidotio-sync/home/kai/.local/share/fish/fish_history

mkdir -p ~/.config/hippocampus/cluster
[ -f ~/.config/hippocampus/cluster/env.sh ] && gcloud storage cp ~/.config/hippocampus/cluster/env.sh gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh
gcloud storage cp gs://kaidotio-sync/home/kai/.config/hippocampus/cluster/env.sh ~/.config/hippocampus/cluster/env.sh

mkdir -p ~/brain
options=()
if [ "$(find -L ~/brain -type f | wc -l)" -ne 0 ]; then
  options=(--delete-unmatched-destination-objects)
fi
gcloud storage rsync ~/brain gs://kaidotio-sync/home/kai/brain --recursive "${options[@]}"
gcloud storage rsync gs://kaidotio-sync/home/kai/brain ~/brain --recursive

mkdir -p ~/.secrets
options=()
if [ "$(find -L ~/.secrets -type f | wc -l)" -ne 0 ]; then
  options=(--delete-unmatched-destination-objects)
fi
gcloud storage rsync ~/.secrets gs://kaidotio-sync/home/kai/.secrets --recursive "${options[@]}"
gcloud storage rsync gs://kaidotio-sync/home/kai/.secrets ~/.secrets --recursive

mkdir -p ~/.vault
if [ -n "$RAILS_MASTER_KEY" ]; then
  find ~ -maxdepth 1 -type f -name ".vault-strip.*" -delete
  password=$(printf '%s' "$RAILS_MASTER_KEY" | rclone obscure -)
  if [ "$(find -L ~/.vault -type f | wc -l)" -eq 0 ]; then
    vault copy vault: ~/.vault
  else
    status=0
    remote_files=$(vault lsl --fast-list vault:) || status=$?
    # 3 is rclone's directory not found, which the first run against an empty prefix returns
    if [ "$status" -ne 0 ] && [ "$status" -ne 3 ]; then
      exit "$status"
    fi
    local_files=$(rclone lsl ~/.vault)
    if [ -z "$local_files" ]; then
      echo "no file listed under ~/.vault" 1>&2
      exit 1
    fi
    upload=$(LC_ALL=C comm -23 <(LC_ALL=C sort <<< "$local_files") <(LC_ALL=C sort <<< "$remote_files") | sed 's/^ *[0-9]* [^ ]* [^ ]* //')
    remove=$(LC_ALL=C comm -13 <(sed 's/^ *[0-9]* [^ ]* [^ ]* //' <<< "$local_files" | LC_ALL=C sort) <(sed 's/^ *[0-9]* [^ ]* [^ ]* //' <<< "$remote_files" | LC_ALL=C sort))
    if [ -n "$upload" ]; then
      while IFS= read -r file; do
        strip_metadata ~/.vault/"$file"
      done <<< "$upload"
      vault copy --files-from-raw <(printf '%s\n' "$upload") ~/.vault vault:
    fi
    if [ -n "$remove" ]; then
      vault delete --files-from-raw <(printf '%s\n' "$remove") vault:
    fi
  fi
fi

# Local transcripts have been lost with no cause identified
mkdir -p ~/.config/claudex/config/projects/-opt-hippocampus
gcloud storage rsync ~/.config/claudex/config/projects/-opt-hippocampus gs://kaidotio-sync/home/kai/.config/claudex/config/projects/-opt-hippocampus --recursive
