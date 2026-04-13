#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

mkdir -p ~/.asdf
(
  cd ~/.asdf
  git init
  if git remote get-url origin >/dev/null 2>&1; then
    git remote set-url origin https://github.com/asdf-vm/asdf.git
  else
    git remote add origin https://github.com/asdf-vm/asdf.git
  fi
  git pull origin v0.14.1 --depth 1
)

cat <<EOS | sudo tee /etc/profile.d/asdf.sh > /dev/null
. /home/${USER}/.asdf/asdf.sh
EOS
